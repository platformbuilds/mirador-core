package cabundle

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/platformbuilds/mirador-core/internal/utils/fswatcher"
)

// Manager watches an optional CA bundle file and exposes a certificate pool built from it.
type Logger interface {
	Info(msg string, fields ...interface{})
	Warn(msg string, fields ...interface{})
}

type Manager struct {
	path     string
	logger   Logger
	onChange func()

	mu   sync.RWMutex
	pool *x509.CertPool

	watcher *fswatcher.Watcher
	stopCh  chan struct{}
	doneCh  chan struct{}
}

// NewManager creates a new Manager. When path is empty no watcher is started.
func NewManager(path string, log Logger, onChange func()) (*Manager, error) {
	cleanPath := filepath.Clean(path)
	mgr := &Manager{
		path:     cleanPath,
		logger:   log,
		onChange: onChange,
	}

	if cleanPath == "" {
		return mgr, nil
	}

	if err := mgr.reloadBundle(); err != nil {
		return nil, fmt.Errorf("load CA bundle %s: %w", cleanPath, err)
	}

	watcher, err := fswatcher.New()
	if err != nil {
		return nil, fmt.Errorf("create fsnotify watcher: %w", err)
	}
	mgr.watcher = watcher
	mgr.stopCh = make(chan struct{})
	mgr.doneCh = make(chan struct{})

	dir := filepath.Dir(cleanPath)
	if err := watcher.Add(dir); err != nil {
		if closeErr := watcher.Close(); closeErr != nil {
			log.Warn("failed to close CA bundle watcher after add failure", "error", closeErr)
		}
		return nil, fmt.Errorf("watch directory %s: %w", dir, err)
	}

	go mgr.watchLoop()
	log.Info("LDAP CA bundle loaded", "path", cleanPath)
	return mgr, nil
}

// RootCAs returns the current certificate pool. May be nil when no bundle configured.
func (m *Manager) RootCAs() *x509.CertPool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.pool
}

// TLSConfig builds a tls.Config fragment with the managed pool and skipVerify setting.
// Callers may further customize the returned config.
func (m *Manager) TLSConfig(skipVerify bool) *tls.Config {
	cfg := &tls.Config{MinVersion: tls.VersionTLS12}
	if skipVerify {
		cfg.InsecureSkipVerify = true
	}
	if pool := m.RootCAs(); pool != nil {
		cfg.RootCAs = pool
	}
	return cfg
}

// ForceReload reloads the bundle from disk immediately.
func (m *Manager) ForceReload() error {
	if m.path == "" {
		return nil
	}
	return m.reloadBundle()
}

// Close stops the watcher and releases resources.
func (m *Manager) Close() error {
	if m.watcher == nil {
		return nil
	}
	close(m.stopCh)
	err := m.watcher.Close()
	<-m.doneCh
	return err
}

func (m *Manager) watchLoop() {
	defer close(m.doneCh)

	for {
		select {
		case event := <-m.watcher.Events:
			if !m.isRelevant(event) {
				continue
			}
			if err := m.reloadWithRetries(); err != nil {
				m.logger.Warn("LDAP CA bundle reload failed", "path", m.path, "error", err)
				continue
			}
			m.logger.Info("LDAP CA bundle reloaded", "path", m.path)
			if m.onChange != nil {
				m.onChange()
			}
		case err := <-m.watcher.Errors:
			m.logger.Warn("LDAP CA bundle watcher error", "error", err)
		case <-m.stopCh:
			return
		}
	}
}

func (m *Manager) reloadWithRetries() error {
	const (
		attempts = 5
		delay    = 200 * time.Millisecond
	)

	var lastErr error
	for i := 0; i < attempts; i++ {
		if err := m.reloadBundle(); err != nil {
			lastErr = err
			time.Sleep(delay)
			continue
		}
		return nil
	}
	return lastErr
}

func (m *Manager) reloadBundle() error {
	pool, err := loadBundle(m.path)
	if err != nil {
		return err
	}
	m.mu.Lock()
	m.pool = pool
	m.mu.Unlock()
	return nil
}

func (m *Manager) isRelevant(event fswatcher.Event) bool {
	if event.Name == "" {
		return true
	}

	eventPath := filepath.Clean(event.Name)
	if eventPath == m.path {
		return true
	}

	return filepath.Dir(eventPath) == filepath.Dir(m.path)
}

var (
	errInvalidPEMData       = errors.New("invalid PEM data in CA bundle")
	errUnexpectedPEMBlock   = errors.New("unexpected PEM block type")
	errNoCertificatesInPool = errors.New("no certificates found in CA bundle")
)

const certificateBlockType = "CERTIFICATE"

func loadBundle(path string) (*x509.CertPool, error) {
	if path == "" {
		return nil, nil
	}

	cleanPath := filepath.Clean(path)
	if !filepath.IsAbs(cleanPath) {
		absPath, err := filepath.Abs(cleanPath)
		if err != nil {
			return nil, fmt.Errorf("resolve CA bundle path: %w", err)
		}
		cleanPath = absPath
	}

	data, err := os.ReadFile(cleanPath)
	if err != nil {
		return nil, fmt.Errorf("read CA bundle: %w", err)
	}

	pool, err := x509.SystemCertPool()
	if err != nil || pool == nil {
		pool = x509.NewCertPool()
	}

	rest := data
	added := false
	for len(rest) > 0 {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			if len(bytes.TrimSpace(rest)) == 0 {
				break
			}
			return nil, errInvalidPEMData
		}
		if block.Type != certificateBlockType {
			return nil, fmt.Errorf("%w: %s", errUnexpectedPEMBlock, block.Type)
		}

		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("parse certificate: %w", err)
		}
		pool.AddCert(cert)
		added = true
	}

	if !added {
		return nil, errNoCertificatesInPool
	}

	return pool, nil
}
