# Package `store` (`internal/store`)

`internal/store` manages persistent state serialization for download sessions on disk.

## Purpose

The package is responsible for:
- Initializing the storage directory under the user's home folder (`~/.downloader/sessions`).
- Saving and loading JSON session files containing complete file metadata, download offsets, statuses, and retry counters.
- Performing atomic writes (writing to temporary files and renaming) to prevent corruption during unexpected shutdowns or crashes.
- Listing saved sessions for inspection via the CLI.

## Key Types & Methods

- [`Store`](store.go#L15): Manages session file reading and writing.
- [`DefaultStore() (*Store, error)`](store.go#L21): Initializes default storage directory at `~/.downloader/sessions`.
- [`SaveSession(sess *model.Session) error`](store.go#L40): Atomically serializes and saves session data to disk.
- [`LoadSession(name string) (*model.Session, error)`](store.go#L62): Loads and parses an existing session by name.
- [`ListSessions() ([]*model.Session, error)`](store.go#L80): Reads and enumerates all saved session files in the storage directory.
