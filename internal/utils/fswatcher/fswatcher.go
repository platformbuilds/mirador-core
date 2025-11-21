package fswatcher

// Deprecated: The fsnotify-based watcher was removed. This package remains as
// a stub to avoid import churn; no live file-watching is performed.

// Event is a minimal struct retained for compatibility; it is not populated
// because live watching is disabled.
type Event struct {
	Name string
}

// Watcher is a noop placeholder.
type Watcher struct{}

// New returns a nil watcher and no error. Callers should not expect live
// filesystem events; the CA bundle manager no longer uses a watcher.
func New() (*Watcher, error) {
	return nil, nil
}
