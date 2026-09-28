# Package `dashboard` (`internal/dashboard`)

`internal/dashboard` implements an interactive, multi-line ANSI terminal dashboard and headless interval logger for real-time progress visualization.

## Purpose

The package is responsible for:
- Detecting whether stdout is connected to an interactive TTY.
- Rendering progress bars, transfer rates, stream statuses (`WARMING_UP`, `STABLE`, `STALLED`), and completed jobs without flicker using ANSI cursor repositions.
- Falling back to a clean interval log when running inside headless environments (such as CI/CD runners or redirect pipes).

## Key Types & Methods

- [`Dashboard`](dashboard.go#L14): Terminal dashboard controller managing UI refresh cycles and terminal state.
- [`NewDashboard(allJobs []*model.FileJob, workersCount int) *Dashboard`](dashboard.go#L26): Factory function detecting TTY capabilities and initializing the dashboard.
- [`Start(renderInterval time.Duration)`](dashboard.go#L46): Spawns the background render ticker loop.
- [`Stop()`](dashboard.go#L63): Halts the render loop and flushes the final display state.
- [`SetActiveJobs(jobs []*model.FileJob)`](dashboard.go#L39): Synchronizes the active jobs currently being transferred by worker slots.
- [`Render()`](dashboard.go#L68): Renders the current state to stdout using either TTY escape sequences or fallback logs.
