# Package `queue` (`internal/queue`)

`internal/queue` provides a thread-safe task queue with Shortest-Job-First (SJF) scheduling and dynamic demotion support.

## Purpose

The package is responsible for:
- Prioritizing smaller files first to minimize average waiting times.
- Placing jobs with unknown sizes (`TotalSize == -1`) at the end of the queue.
- Re-enqueuing demoted or paused jobs dynamically at runtime.
- Coordinating worker goroutines using sync.Cond wait-signal synchronization.

## Key Types & Methods

- [`Queue`](queue.go#L11): Thread-safe queue protecting job slices with mutex and condition variable synchronization.
- [`NewQueue(initial []*model.FileJob) *Queue`](queue.go#L19): Creates and initializes a queue pre-sorted with jobs.
- [`SortAndSet(jobs []*model.FileJob)`](queue.go#L33): Sorts jobs in ascending order of known file size (SJF).
- [`Pop() (*model.FileJob, bool)`](queue.go#L60): Blocks until a job is ready or the queue closes, returning the next available job.
- [`Push(job *model.FileJob)`](queue.go#L76): Re-enqueues a job and signals waiting workers.
- [`Demote(job *model.FileJob)`](queue.go#L87): Demotes a job to the tail of the queue and marks its status as `StatusDemoted`.
- [`Len() int`](queue.go#L99): Returns the count of remaining queued items.
- [`Close()`](queue.go#L106): Closes the queue and unblocks all waiting worker threads.
