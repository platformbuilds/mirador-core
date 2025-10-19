package logger

import "testing"

func TestLogger_BasicLevels(t *testing.T) {
	l := New("debug")
	if l == nil {
		t.Fatalf("logger nil")
	}
	l.Debug("dbg", "k", 1)
	l.Info("info")
	l.Warn("warn")
	l.Error("err")
}
