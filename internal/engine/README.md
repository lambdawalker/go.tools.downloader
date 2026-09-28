# Package `engine` (`internal/engine`)

`internal/engine` is the central orchestrator connecting workers, queue scheduling, watchdog monitoring, session persistence, and UI rendering.

## Purpose

The package is responsible for:
- Initializing the worker pool based on user configuration.
- Probing URLs prior to download and initializing jobs.
- Feeding jobs into the Shortest-Job-First (SJF) queue.
- Coordinating transfer execution with rate limiting and retry backoff.
- Hooking active streams into the watchdog to detect stalls and slow transfers.
- Saving session states to disk on interval and on clean termination.

## Key Types & Methods

- [`Config`](engine.go#L24): Configuration parameters governing worker pool size, output directory, rate limits, and timeouts.
- [`Engine`](engine.go#L34): Central orchestrator coordinating workers, watchdog, and storage.
- [`NewEngine(cfg Config, st *store.Store, authMgr *auth.Manager, vault *auth.Vault) *Engine`](engine.go#L43): Creates and wires an Engine instance.
- [`Run(ctx context.Context, session *model.Session) error`](engine.go#L64): Executes the complete lifecycle of a download session.
