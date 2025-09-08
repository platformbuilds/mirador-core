package logger

import "testing"

func TestLogger_NewAndMethods(t *testing.T) {
    l := New("error")
    l.Info("info")
    l.Warn("warn")
    l.Error("error")
    l.Debug("debug")
}

