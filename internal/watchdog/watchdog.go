// Package watchdog monitors active download streams, evaluates peer performance,
// detects stalled transfers, and triggers dynamic demotions to optimize queue throughput.
package watchdog

import (
	"context"
	"sync"
	"time"

	"downloader/internal/model"
	"downloader/internal/queue"
)

// ActiveStream represents an in-flight download stream and its associated cancellation handle.
type ActiveStream struct {
	Job        *model.FileJob
	CancelFunc context.CancelFunc
	StartedAt  time.Time
}

// Watchdog tracks active worker streams and implements adaptive demotion policies.
type Watchdog struct {
	mu                sync.RWMutex
	streams           map[string]*ActiveStream
	q                 *queue.Queue
	lastStreamStarted time.Time
	warmupDuration    time.Duration
	stallThreshold    time.Duration
	stableHoldWindow  time.Duration
	safetyValveLimit  time.Duration
}

// NewWatchdog initializes a Watchdog configured with default stall and warmup detection windows.
func NewWatchdog(q *queue.Queue) *Watchdog {
	return &Watchdog{
		streams:          make(map[string]*ActiveStream),
		q:                q,
		warmupDuration:   20 * time.Second,
		stallThreshold:   15 * time.Second,
		stableHoldWindow: 10 * time.Second,
		safetyValveLimit: 60 * time.Second,
	}
}

// RegisterStream associates an active job stream and cancellation callback with the watchdog.
func (w *Watchdog) RegisterStream(job *model.FileJob, cancel context.CancelFunc) {
	w.mu.Lock()
	defer w.mu.Unlock()

	now := time.Now()
	w.streams[job.ID] = &ActiveStream{
		Job:        job,
		CancelFunc: cancel,
		StartedAt:  now,
	}
	job.DownloadStartedAt = now
	job.LastByteReceived = now
	job.SetStreamState(model.StateConnecting)
	w.lastStreamStarted = now
}

// UnregisterStream removes the finished or canceled job stream from watchdog tracking.
func (w *Watchdog) UnregisterStream(jobID string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	delete(w.streams, jobID)
}

// ActiveCount returns the number of streams currently being monitored.
func (w *Watchdog) ActiveCount() int {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return len(w.streams)
}

// Tick evaluates stream health, detects stalled downloads, and identifies candidates for demotion.
func (w *Watchdog) Tick() []*model.FileJob {
	w.mu.Lock()
	defer w.mu.Unlock()

	now := time.Now()
	var demotions []*model.FileJob

	if len(w.streams) == 0 {
		return nil
	}

	// 1. Update stream states and check for stalls
	allStable := true
	var totalSpeed float64
	var activeSpeeds []float64

	for _, s := range w.streams {
		job := s.Job
		sinceLastByte := now.Sub(job.LastByteReceived)
		runningTime := now.Sub(s.StartedAt)

		// Check stall
		if sinceLastByte >= w.stallThreshold {
			job.SetStreamState(model.StateStalled)
			allStable = false

			// Condition C: Stall Demotion
			if w.eligibleForDemotion(job) {
				demotions = append(demotions, job)
				s.CancelFunc()
				continue
			}
		} else if runningTime < w.warmupDuration {
			job.SetStreamState(model.StateWarmingUp)
			allStable = false
		} else {
			job.SetStreamState(model.StateStable)
		}

		snapBytes, _, speed, state, _ := job.GetSnapshot()
		_ = snapBytes
		if state == model.StateStable || state == model.StateWarmingUp {
			totalSpeed += speed
			activeSpeeds = append(activeSpeeds, speed)
		}
	}

	// 2. Peer Speed Average Calculation
	var peerAvg float64
	if len(activeSpeeds) > 0 {
		peerAvg = totalSpeed / float64(len(activeSpeeds))
	}

	// Check if queue has waiting items
	hasPendingQueue := w.q.Len() > 0
	if !hasPendingQueue {
		// Nothing to gain by demoting if no other downloads are waiting
		return demotions
	}

	// 3. Evaluate Condition A: All Active Streams Stable for hold duration
	stableHoldSatisfied := allStable && (now.Sub(w.lastStreamStarted) >= w.stableHoldWindow)

	for _, s := range w.streams {
		job := s.Job
		// Skip if already marked for demotion
		if containsJob(demotions, job.ID) || !w.eligibleForDemotion(job) {
			continue
		}

		runningTime := now.Sub(s.StartedAt)
		_, _, speed, state, _ := job.GetSnapshot()

		// Condition A: Stable peer comparison (< 2/3 peer average)
		if stableHoldSatisfied && state == model.StateStable {
			if peerAvg > 1024 && speed < (2.0/3.0)*peerAvg {
				demotions = append(demotions, job)
				s.CancelFunc()
				w.lastStreamStarted = now // Cooldown reset
				continue
			}
		}

		// Condition B: 1-minute safety valve (failing to ramp up / chronically slow)
		if runningTime >= w.safetyValveLimit {
			if peerAvg > 1024 && speed < (2.0/3.0)*peerAvg {
				demotions = append(demotions, job)
				s.CancelFunc()
				w.lastStreamStarted = now // Cooldown reset
				continue
			}
		}
	}

	return demotions
}

// eligibleForDemotion determines whether a job is allowed to be preempted and demoted.
func (w *Watchdog) eligibleForDemotion(job *model.FileJob) bool {
	// Must not have been demoted already
	if job.DemotedOnce {
		return false
	}
	// Must be resumable
	if !job.AcceptRanges {
		return false
	}
	// If total size is known, check remaining work threshold
	if job.TotalSize > 0 {
		remaining := job.TotalSize - job.DownloadedBytes
		progress := float64(job.DownloadedBytes) / float64(job.TotalSize)
		// Don't demote if remaining <= 10MB or > 80% completed
		if remaining <= 10*1024*1024 || progress >= 0.80 {
			return false
		}
	}
	return true
}

// containsJob checks whether a job with the specified id is already present in list.
func containsJob(list []*model.FileJob, id string) bool {
	for _, item := range list {
		if item.ID == id {
			return true
		}
	}
	return false
}
