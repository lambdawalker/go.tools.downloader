# Package `watchdog` (`internal/watchdog`)

`internal/watchdog` actively monitors concurrent download streams, detects stalls and slow peer performance, and triggers adaptive demotions.

## Purpose

The package is responsible for:
- Tracking live network streams and their respective cancellation contexts.
- Enforcing stall thresholds (default 15 seconds without receiving data).
- Evaluating peer transfer speeds against an adaptive benchmark: demoting streams running at less than two-thirds (2/3) of peer average after stability windows or safety limits.
- Preventing head-of-line blocking by pre-empting slow resumable transfers and allowing queued jobs to make progress.
- Guarding near-complete files (>80% downloaded or under 10 MB remaining) from being demoted.

## Key Types & Methods

- [`ActiveStream`](file:///D:/dev/downloader/internal/watchdog/watchdog.go#L11): Represents an in-flight stream with cancellation function and start timestamp.
- [`Watchdog`](file:///D:/dev/downloader/internal/watchdog/watchdog.go#L18): Oversees stream states and executes demotion evaluations.
- [`NewWatchdog(q *queue.Queue) *Watchdog`](file:///D:/dev/downloader/internal/watchdog/watchdog.go#L30): Instantiates a Watchdog configured with default timing thresholds.
- [`RegisterStream(job *model.FileJob, cancel context.CancelFunc)`](file:///D:/dev/downloader/internal/watchdog/watchdog.go#L42): Registers a running worker stream with the watchdog.
- [`UnregisterStream(jobID string)`](file:///D:/dev/downloader/internal/watchdog/watchdog.go#L58): Removes a completed or terminated stream.
- [`Tick() []*model.FileJob`](file:///D:/dev/downloader/internal/watchdog/watchdog.go#L71): Evaluates stream conditions and returns jobs selected for demotion.
