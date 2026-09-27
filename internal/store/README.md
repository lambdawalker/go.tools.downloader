# Package `store` (`internal/store`)

`internal/store` manages persistent on-disk storage of download sessions as JSON files, enabling pause/resume workflows and state recovery.

## Purpose

The package is responsible for:
- Managing session persistence under the user directory (`~/.downloader/sessions/`).
- Performing atomic writes by creating temporary `.tmp` files and renaming them to ensure zero corruption during unexpected power loss or crashes.
- Serializing and deserializing complete session metadata including job progress, retry counters, and checksums.
- Discovering and listing saved sessions for CLI inspection.

## Key Types & Methods

- [`Store`](file:///D:/dev/downloader/internal/store/store.go#L15): Manages session file reading and writing.
- [`DefaultStore() (*Store, error)`](file:///D:/dev/downloader/internal/store/store.go#L21): Initializes default storage directory at `~/.downloader/sessions`.
- [`SaveSession(sess *model.Session) error`](file:///D:/dev/downloader/internal/store/store.go#L40): Atomically serializes and saves session data to disk.
- [`LoadSession(name string) (*model.Session, error)`](file:///D:/dev/downloader/internal/store/store.go#L62): Loads and parses an existing session by name.
- [`ListSessions() ([]*model.Session, error)`](file:///D:/dev/downloader/internal/store/store.go#L80): Reads and enumerates all saved session files in the storage directory.
