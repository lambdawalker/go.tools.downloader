# Package `watchdog` (`internal/watchdog`)

`internal/watchdog` actively monitors the health, progress, and speeds of ongoing HTTP streams to preemptively mitigate stalls and deprioritize persistently slow transfers.

## Purpose

The package is responsible for:
- Tracking live byte timestamps per active worker stream.
- Marking streams as stalled if no data has been received within the stall timeout window (default 15s).
- Tracking overall peer average speeds and identifying streams running severely below peer throughput (default < 2/3 peer average).
- Demoting underperforming streams dynamically to the back of the SJF queue to let other pending downloads proceed.
- Enforcing an anti-churn policy: files can only be demoted once to prevent infinite loops.

## Key Types & Methods

- [`ActiveStream`](watchdog.go#L11): Represents an in-flight stream with cancellation function and start timestamp.
- [`Watchdog`](watchdog.go#L18): Oversees stream states and executes demotion evaluations.
- [`NewWatchdog(q *queue.Queue) *Watchdog`](watchdog.go#L30): Instantiates a Watchdog configured with default timing thresholds.
- [`RegisterStream(job *model.FileJob, cancel context.CancelFunc)`](watchdog.go#L42): Registers a running worker stream with the watchdog.
- [`UnregisterStream(jobID string)`](watchdog.go#L58): Removes a completed or terminated stream.
- [`Tick() []*model.FileJob`](watchdog.go#L71): Evaluates stream conditions and returns jobs selected for demotion.
