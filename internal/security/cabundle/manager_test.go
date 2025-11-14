package cabundle

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNewManager_WithInvalidBundle(t *testing.T) {
	dir := t.TempDir()
	bundlePath := filepath.Join(dir, "ldap-ca.pem")

	if err := os.WriteFile(bundlePath, []byte("not a cert"), 0o600); err != nil {
		t.Fatalf("write bundle: %v", err)
	}

	if _, err := NewManager(bundlePath, newStubLogger(&strings.Builder{}), nil); err == nil {
		t.Fatalf("expected error for invalid CA bundle")
	}
}

func TestManagerForceReload(t *testing.T) {
	dir := t.TempDir()
	bundlePath := filepath.Join(dir, "ldap-ca.pem")

	if err := os.WriteFile(bundlePath, generateTestCertPEM(t), 0o600); err != nil {
		t.Fatalf("write bundle: %v", err)
	}

	logBuf := &strings.Builder{}
	mgr, err := NewManager(bundlePath, newStubLogger(logBuf), nil)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	t.Cleanup(func() {
		if err := mgr.Close(); err != nil {
			t.Fatalf("close manager: %v", err)
		}
	})

	if mgr.RootCAs() == nil {
		t.Fatalf("expected RootCAs to be populated")
	}

	// Update bundle with a new certificate and force reload
	time.Sleep(10 * time.Millisecond)
	if err := os.WriteFile(bundlePath, generateTestCertPEM(t), 0o600); err != nil {
		t.Fatalf("update bundle: %v", err)
	}

	if err := mgr.ForceReload(); err != nil {
		t.Fatalf("ForceReload: %v", err)
	}
}

func generateTestCertPEM(t *testing.T) []byte {
	t.Helper()

	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	template := &x509.Certificate{
		SerialNumber:          big.NewInt(time.Now().UnixNano()),
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		IsCA:                  true,
		Subject:               pkix.Name{CommonName: "mirador-test-ca"},
		BasicConstraintsValid: true,
	}

	der, err := x509.CreateCertificate(rand.Reader, template, template, &priv.PublicKey, priv)
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}

	var buf bytes.Buffer
	if err := pem.Encode(&buf, &pem.Block{Type: "CERTIFICATE", Bytes: der}); err != nil {
		t.Fatalf("encode pem: %v", err)
	}

	return buf.Bytes()
}

type stubLogger struct {
	buf *strings.Builder
}

func newStubLogger(out *strings.Builder) Logger {
	return &stubLogger{buf: out}
}

func (l *stubLogger) Info(msg string, fields ...interface{}) {
	if l.buf == nil {
		return
	}
	l.buf.WriteString("[INFO] " + msg + "\n")
}

func (l *stubLogger) Warn(msg string, fields ...interface{}) {
	if l.buf == nil {
		return
	}
	l.buf.WriteString("[WARN] " + msg + "\n")
}
