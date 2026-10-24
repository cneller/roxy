package proxy

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

// writePidFile writes a raw pid so a test can plant another process's pid,
// which WritePidFile cannot do because it always writes the caller's own.
func writePidFile(t *testing.T, configDir string, pid int) {
	t.Helper()
	if err := os.WriteFile(PidFile(configDir), []byte(strconv.Itoa(pid)), 0o600); err != nil {
		t.Fatalf("write pid file: %v", err)
	}
}

func TestRemovePidFileLeavesALiveForeignProxyAlone(t *testing.T) {
	configDir := t.TempDir()
	foreign := os.Getppid()
	if !processAlive(foreign) {
		t.Fatalf("parent pid %d is not alive; the test needs a live foreign pid", foreign)
	}
	writePidFile(t, configDir, foreign)
	state := filepath.Join(configDir, "proxy.state.json")
	if err := os.WriteFile(state, []byte(`{"pid":1}`), 0o600); err != nil {
		t.Fatalf("write state: %v", err)
	}

	RemovePidFile(configDir)

	if _, err := os.Stat(PidFile(configDir)); err != nil {
		t.Errorf("pid file held by live pid %d was removed: %v", foreign, err)
	}
	if _, err := os.Stat(state); err != nil {
		t.Errorf("state file of a live foreign proxy was removed: %v", err)
	}
}

func TestRemovePidFileClearsOwnAndStaleFiles(t *testing.T) {
	t.Run("own pid", func(t *testing.T) {
		configDir := t.TempDir()
		if err := WritePidFile(configDir); err != nil {
			t.Fatalf("write own pid: %v", err)
		}
		RemovePidFile(configDir)
		if _, err := os.Stat(PidFile(configDir)); !os.IsNotExist(err) {
			t.Errorf("own pid file survived removal: %v", err)
		}
	})

	t.Run("dead pid", func(t *testing.T) {
		configDir := t.TempDir()
		// PID 2^22 + 1 cannot name a live process: it is past both the
		// kernel's PID_MAX and launchd's default pid limit.
		writePidFile(t, configDir, 1<<22+1)
		RemovePidFile(configDir)
		if _, err := os.Stat(PidFile(configDir)); !os.IsNotExist(err) {
			t.Errorf("stale pid file survived removal: %v", err)
		}
	})

	t.Run("no pid file", func(t *testing.T) {
		configDir := t.TempDir()
		RemovePidFile(configDir)
		if _, err := os.Stat(PidFile(configDir)); !os.IsNotExist(err) {
			t.Errorf("RemovePidFile created a pid file: %v", err)
		}
	})
}
