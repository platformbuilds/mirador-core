import (
	"context"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/mirador/core/pkg/logger"
)

type ConfigWatcher struct {
	config     *Config
	configPath string
	logger     logger.Logger
	mu         sync.RWMutex
	watchers   []func(*Config)
	stopCh     chan struct{}
}

func NewConfigWatcher(configPath string, logger logger.Logger) *ConfigWatcher {
	return &ConfigWatcher{
		configPath: configPath,
		logger:     logger,
		watchers:   make([]func(*Config), 0),
		stopCh:     make(chan struct{}),
	}
}

// Start begins watching for configuration file changes
func (w *ConfigWatcher) Start(ctx context.Context) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("failed to create file watcher: %w", err)
	}
	defer watcher.Close()

	// Add configuration file to watcher
	if err := watcher.Add(w.configPath); err != nil {
		return fmt.Errorf("failed to watch config file: %w", err)
	}

	w.logger.Info("Configuration watcher started", "configPath", w.configPath)

	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return nil
			}

			if event.Op&fsnotify.Write == fsnotify.Write {
				w.logger.Info("Configuration file changed, reloading...", "file", event.Name)
				
				// Reload configuration
				if err := w.reloadConfig(); err != nil {
					w.logger.Error("Failed to reload configuration", "error", err)
					continue
				}

				// Notify watchers
				w.notifyWatchers()
			}

		case err, ok := <-watcher.Errors:
			if !ok {
				return nil
			}
			w.logger.Error("Configuration watcher error", "error", err)

		case <-ctx.Done():
			w.logger.Info("Configuration watcher stopping")
			return nil

		case <-w.stopCh:
			w.logger.Info("Configuration watcher stopped")
			return nil
		}
	}
}

// RegisterWatcher adds a callback for configuration changes
func (w *ConfigWatcher) RegisterWatcher(callback func(*Config)) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.watchers = append(w.watchers, callback)
}

// GetConfig returns the current configuration (thread-safe)
func (w *ConfigWatcher) GetConfig() *Config {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.config
}

// Stop stops the configuration watcher
func (w *ConfigWatcher) Stop() {
	close(w.stopCh)
}

func (w *ConfigWatcher) reloadConfig() error {
	newConfig, err := Load()
	if err != nil {
		return err
	}

	w.mu.Lock()
	w.config = newConfig
	w.mu.Unlock()

	w.logger.Info("Configuration reloaded successfully")
	return nil
}

func (w *ConfigWatcher) notifyWatchers() {
	w.mu.RLock()
	config := w.config
	watchers := make([]func(*Config), len(w.watchers))
	copy(watchers, w.watchers)
	w.mu.RUnlock()

	for _, watcher := range watchers {
		go func(w func(*Config)) {
			defer func() {
				if r := recover(); r != nil {
					// Log panic but don't crash the watcher
					fmt.Printf("Configuration watcher panic: %v\n", r)
				}
			}()
			w(config)
		}(watcher)
	}
}
