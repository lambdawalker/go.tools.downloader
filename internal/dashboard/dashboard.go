// Package dashboard provides terminal UI visualization for download progress,
// worker status, bandwidth speeds, and queue metrics.
package dashboard

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"downloader/internal/model"
)

// Dashboard represents the terminal dashboard renderer for tracking download progress across concurrent workers.
type Dashboard struct {
	mu           sync.Mutex
	isTTY        bool
	linesDrawn   int
	lastLogTime  time.Time
	activeJobs   []*model.FileJob
	allJobs      []*model.FileJob
	stopChan     chan struct{}
	workersCount int
}

// NewDashboard creates a new Dashboard instance, detecting whether stdout is an interactive character device (TTY).
func NewDashboard(allJobs []*model.FileJob, workersCount int) *Dashboard {
	fileInfo, err := os.Stdout.Stat()
	isTTY := false
	if err == nil {
		isTTY = (fileInfo.Mode() & os.ModeCharDevice) != 0
	}

	return &Dashboard{
		isTTY:        isTTY,
		allJobs:      allJobs,
		workersCount: workersCount,
		stopChan:     make(chan struct{}),
	}
}

// SetActiveJobs updates the slice of jobs currently assigned to active workers for display.
func (d *Dashboard) SetActiveJobs(jobs []*model.FileJob) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.activeJobs = jobs
}

// Start begins periodic rendering of the dashboard at the specified interval in a background goroutine.
func (d *Dashboard) Start(renderInterval time.Duration) {
	ticker := time.NewTicker(renderInterval)
	go func() {
		for {
			select {
			case <-ticker.C:
				d.Render()
			case <-d.stopChan:
				ticker.Stop()
				d.Render() // final draw
				return
			}
		}
	}()
}

// Stop terminates the background rendering loop and triggers a final render flush.
func (d *Dashboard) Stop() {
	close(d.stopChan)
}

// Render paints the current status of all downloads and active workers to standard output.
func (d *Dashboard) Render() {
	d.mu.Lock()
	defer d.mu.Unlock()

	var pending, completed, demoted, failed int
	for _, j := range d.allJobs {
		switch j.Status {
		case model.StatusCompleted:
			completed++
		case model.StatusFailed:
			failed++
		case model.StatusDemoted:
			demoted++
		default:
			pending++
		}
	}

	if d.isTTY {
		d.renderTTY(pending, completed, demoted, failed)
	} else {
		d.renderNonTTY(pending, completed, demoted, failed)
	}
}

// renderTTY draws an interactive ANSI-formatted multi-line display with progress bars and color-coded statuses.
func (d *Dashboard) renderTTY(pending, completed, demoted, failed int) {
	// Erase previous lines
	if d.linesDrawn > 0 {
		fmt.Printf("\033[%dA\033[J", d.linesDrawn)
	}

	lines := 0
	fmt.Printf("\033[1;36m=== Go Concurrent Resumable Downloader ===\033[0m\n")
	lines++

	fmt.Printf("Workers: %d | Pending: %d | Completed: %d | Demoted: %d | Failed: %d\n",
		d.workersCount, pending, completed, demoted, failed)
	lines++
	fmt.Println(strings.Repeat("-", 75))
	lines++

	for i := 0; i < d.workersCount; i++ {
		if i < len(d.activeJobs) && d.activeJobs[i] != nil {
			j := d.activeJobs[i]
			downloaded, total, speed, state, _ := j.GetSnapshot()

			name := j.ResolvedFilename
			if len(name) > 22 {
				name = name[:19] + "..."
			}

			percentStr := "??%"
			progressBar := "[??????????]"
			if total > 0 {
				pct := float64(downloaded) / float64(total) * 100.0
				percentStr = fmt.Sprintf("%5.1f%%", pct)
				bars := int(pct / 10.0)
				if bars > 10 {
					bars = 10
				}
				progressBar = fmt.Sprintf("[%s%s]", strings.Repeat("=", bars), strings.Repeat(" ", 10-bars))
			}

			stateColor := "\033[33m" // Yellow (Warming up / connecting)
			if state == model.StateStable {
				stateColor = "\033[32m" // Green
			} else if state == model.StateStalled {
				stateColor = "\033[31m" // Red
			}

			fmt.Printf("Worker #%d: %-22s %s %s %8s/%-8s %10s %s[%s]\033[0m\n",
				i+1, name, progressBar, percentStr,
				model.FormatBytes(downloaded), model.FormatBytes(total),
				model.FormatSpeed(speed), stateColor, state)
		} else {
			fmt.Printf("Worker #%d: [Idle]\n", i+1)
		}
		lines++
	}

	fmt.Println(strings.Repeat("-", 75))
	lines++

	d.linesDrawn = lines
}

// renderNonTTY outputs periodic text log updates for non-interactive or redirected terminal environments.
func (d *Dashboard) renderNonTTY(pending, completed, demoted, failed int) {
	if time.Since(d.lastLogTime) < 5*time.Second {
		return
	}
	d.lastLogTime = time.Now()

	fmt.Printf("[%s] Status: Pending: %d, Completed: %d, Demoted: %d, Failed: %d\n",
		time.Now().Format("15:04:05"), pending, completed, demoted, failed)
}
