# Package `dashboard` (`internal/dashboard`)

`internal/dashboard` provides real-time terminal UI rendering to display download progress, worker states, and queue statistics.

## Purpose

The package is responsible for:
- Detecting whether stdout is an interactive character terminal (TTY) or redirected/pipe output.
- Rendering an ANSI-escaped, multi-line status board with live progress bars, speed gauges, and color-coded stream states (`STABLE`, `WARMING_UP`, `STALLED`).
- Providing clean periodic log output for non-TTY or CI/CD headless environments.
- Running a background ticker loop that periodically repaints worker statuses and overall session counts.

## Key Types & Methods

- [`Dashboard`](file:///D:/dev/downloader/internal/dashboard/dashboard.go#L14): Terminal dashboard controller managing UI refresh cycles and terminal state.
- [`NewDashboard(allJobs []*model.FileJob, workersCount int) *Dashboard`](file:///D:/dev/downloader/internal/dashboard/dashboard.go#L26): Factory function detecting TTY capabilities and initializing the dashboard.
- [`Start(renderInterval time.Duration)`](file:///D:/dev/downloader/internal/dashboard/dashboard.go#L46): Spawns the background render ticker loop.
- [`Stop()`](file:///D:/dev/downloader/internal/dashboard/dashboard.go#L63): Halts the render loop and flushes the final display state.
- [`SetActiveJobs(jobs []*model.FileJob)`](file:///D:/dev/downloader/internal/dashboard/dashboard.go#L39): Synchronizes the active jobs currently being transferred by worker slots.
- [`Render()`](file:///D:/dev/downloader/internal/dashboard/dashboard.go#L68): Renders the current state to stdout using either TTY escape sequences or fallback logs.
