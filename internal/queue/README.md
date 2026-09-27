# Package `queue` (`internal/queue`)

`internal/queue` implements a synchronized, thread-safe job queue featuring Shortest-Job-First (SJF) scheduling and runtime demotion.

## Purpose

The package is responsible for:
- Prioritizing downloads using Shortest-Job-First (SJF) ordering based on known file sizes discovered during the probing phase.
- Placing downloads with unknown sizes at the bottom of the queue.
- Providing blocking condition-variable (`sync.Cond`) pop semantics for concurrent worker goroutines.
- Supporting dynamic demotion of slow or stalled streams, appending demoted jobs to the tail of the queue.
- Enabling clean shutdown signaling when all downloads finish or cancellation occurs.

## Key Types & Methods

- [`Queue`](file:///D:/dev/downloader/internal/queue/queue.go#L11): Thread-safe queue protecting job slices with mutex and condition variable synchronization.
- [`NewQueue(initial []*model.FileJob) *Queue`](file:///D:/dev/downloader/internal/queue/queue.go#L19): Creates and initializes a queue pre-sorted with jobs.
- [`SortAndSet(jobs []*model.FileJob)`](file:///D:/dev/downloader/internal/queue/queue.go#L33): Sorts jobs in ascending order of known file size (SJF).
- [`Pop() (*model.FileJob, bool)`](file:///D:/dev/downloader/internal/queue/queue.go#L60): Blocks until a job is ready or the queue closes, returning the next available job.
- [`Push(job *model.FileJob)`](file:///D:/dev/downloader/internal/queue/queue.go#L76): Re-enqueues a job and signals waiting workers.
- [`Demote(job *model.FileJob)`](file:///D:/dev/downloader/internal/queue/queue.go#L87): Demotes a job to the tail of the queue and marks its status as `StatusDemoted`.
- [`Len() int`](file:///D:/dev/downloader/internal/queue/queue.go#L99): Returns the count of remaining queued items.
- [`Close()`](file:///D:/dev/downloader/internal/queue/queue.go#L106): Closes the queue and unblocks all waiting worker threads.
