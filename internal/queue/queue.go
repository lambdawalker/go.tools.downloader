// Package queue implements a thread-safe synchronized job queue supporting
// Shortest-Job-First (SJF) scheduling and runtime job demotion.
package queue

import (
	"sort"
	"sync"

	"downloader/internal/model"
)

// Queue manages concurrent access to pending download jobs.
type Queue struct {
	mu     sync.Mutex
	cond   *sync.Cond
	items  []*model.FileJob
	closed bool
}

// NewQueue constructs a new Queue preloaded and sorted with the initial set of jobs.
func NewQueue(initial []*model.FileJob) *Queue {
	q := &Queue{
		items: make([]*model.FileJob, 0, len(initial)),
	}
	q.cond = sync.NewCond(&q.mu)

	if len(initial) > 0 {
		q.SortAndSet(initial)
	}

	return q
}

// SortAndSet sorts jobs in ascending order of known file size (SJF), placing unknown sizes at the end.
func (q *Queue) SortAndSet(jobs []*model.FileJob) {
	q.mu.Lock()
	defer q.mu.Unlock()

	sorted := make([]*model.FileJob, len(jobs))
	copy(sorted, jobs)

	sort.SliceStable(sorted, func(i, j int) bool {
		sizeI := sorted[i].TotalSize
		sizeJ := sorted[j].TotalSize

		if sizeI > 0 && sizeJ > 0 {
			return sizeI < sizeJ
		}
		if sizeI > 0 && sizeJ <= 0 {
			return true
		}
		if sizeI <= 0 && sizeJ > 0 {
			return false
		}
		return false
	})

	q.items = sorted
	q.cond.Broadcast()
}

// Pop extracts the next job from the queue, blocking if the queue is empty until an item is pushed or closed.
func (q *Queue) Pop() (*model.FileJob, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()

	for len(q.items) == 0 && !q.closed {
		q.cond.Wait()
	}

	if q.closed || len(q.items) == 0 {
		return nil, false
	}

	job := q.items[0]
	q.items = q.items[1:]
	return job, true
}

// Push adds a job back to the end of the queue and signals a waiting worker.
func (q *Queue) Push(job *model.FileJob) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.closed {
		return
	}
	q.items = append(q.items, job)
	q.cond.Signal()
}

// Demote marks a job as demoted, appends it to the end of the queue, and signals a waiting worker.
func (q *Queue) Demote(job *model.FileJob) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.closed {
		return
	}
	job.DemotedOnce = true
	job.Status = model.StatusDemoted
	q.items = append(q.items, job)
	q.cond.Signal()
}

// Len returns the number of pending jobs currently in the queue.
func (q *Queue) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.items)
}

// Close marks the queue as closed and wakes all waiting workers.
func (q *Queue) Close() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.closed = true
	q.cond.Broadcast()
}
