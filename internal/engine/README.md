# Package `engine` (`internal/engine`)

`internal/engine` orchestrates the core execution pipeline, linking all subsystem components together into a unified concurrent download engine.

## Purpose

The package is responsible for:
- Initializing subsystem components (prober, queue, watchdog, downloader, dashboard, and storage).
- Executing the pre-flight URL probing phase to discover file metadata before transferring data.
- Managing the pool of worker goroutines consuming jobs from the prioritized queue.
- Running the watchdog monitoring loop to detect stalled or slow streams and trigger demotions.
- Handling graceful shutdown upon receiving OS termination signals (`SIGINT`, `SIGTERM`), pausing jobs cleanly.
- Persisting session states periodically to disk and outputting final completion summaries.

## Key Types & Functions

- [`Config`](file:///D:/dev/downloader/internal/engine/engine.go#L24): Configuration parameters governing worker pool size, output directory, rate limits, and timeouts.
- [`Engine`](file:///D:/dev/downloader/internal/engine/engine.go#L34): Central orchestrator coordinating workers, watchdog, and storage.
- [`NewEngine(cfg Config, st *store.Store) *Engine`](file:///D:/dev/downloader/internal/engine/engine.go#L43): Creates and wires an Engine instance.
- [`Run(ctx context.Context, session *model.Session) error`](file:///D:/dev/downloader/internal/engine/engine.go#L64): Executes the complete lifecycle of a download session.
