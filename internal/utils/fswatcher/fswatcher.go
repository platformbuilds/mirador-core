package fswatcher

import "github.com/fsnotify/fsnotify"

// Event exposes filesystem watcher events without leaking external dependency across the codebase.
type Event = fsnotify.Event

// Watcher is an alias to fsnotify.Watcher so call sites can rely on a thin wrapper.
type Watcher = fsnotify.Watcher

// New creates a new filesystem watcher. Callers are responsible for closing it.
func New() (*fsnotify.Watcher, error) {
	return fsnotify.NewWatcher()
}
