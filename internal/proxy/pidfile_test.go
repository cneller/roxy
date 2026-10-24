package proxy

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
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

func TestWritePidFileDoesNotReplaceLiveOwner(t *testing.T) {
	configDir := t.TempDir()
	writePidFile(t, configDir, os.Getppid())
	if err := WritePidFile(configDir); err == nil {
		t.Fatal("claimed metadata already owned by a live process")
	}
	if got := ReadPid(configDir); got != os.Getppid() {
		t.Fatalf("owner changed to %d", got)
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

	RemovePidFile(configDir, os.Getpid())

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
		RemovePidFile(configDir, os.Getpid())
		if _, err := os.Stat(PidFile(configDir)); !os.IsNotExist(err) {
			t.Errorf("own pid file survived removal: %v", err)
		}
	})

	t.Run("dead pid", func(t *testing.T) {
		configDir := t.TempDir()
		// PID 2^22 + 1 cannot name a live process: it is past both the
		// kernel's PID_MAX and launchd's default pid limit.
		writePidFile(t, configDir, 1<<22+1)
		RemovePidFile(configDir, 1<<22+1)
		if _, err := os.Stat(PidFile(configDir)); !os.IsNotExist(err) {
			t.Errorf("stale pid file survived removal: %v", err)
		}
	})

	t.Run("no pid file", func(t *testing.T) {
		configDir := t.TempDir()
		RemovePidFile(configDir, os.Getpid())
		if _, err := os.Stat(PidFile(configDir)); !os.IsNotExist(err) {
			t.Errorf("RemovePidFile created a pid file: %v", err)
		}
	})
}

func TestExpectedOwnerCleanupPreservesReplacement(t *testing.T) {
	dir := t.TempDir()
	oldPID := 1<<22 + 1
	writePidFile(t, dir, os.Getpid())
	if err := WriteState(dir, ProxyState{PID: os.Getpid()}); err != nil {
		t.Fatal(err)
	}
	RemovePidFile(dir, oldPID)
	if ReadPid(dir) != os.Getpid() || ReadState(dir) == nil {
		t.Fatal("old stop removed replacement metadata")
	}
}

func TestProcessProbePermissionDeniedIsNotDead(t *testing.T) {
	if !processExistsAfterSignal(syscall.EPERM) {
		t.Fatal("EPERM must preserve ownership")
	}
	if processExistsAfterSignal(syscall.ESRCH) {
		t.Fatal("ESRCH must mean dead")
	}
}

func TestRunBindFailureDoesNotPublishMetadata(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	dir := t.TempDir()
	writePidFile(t, dir, os.Getppid())
	srv := New(Options{RoutesFile: filepath.Join(dir, "routes.json")})
	srv.httpAddr = ln.Addr().String()
	called := false
	err = srv.RunContext(context.Background(), func() error { called = true; return WritePidFile(dir) })
	if err == nil || called {
		t.Fatalf("bind failure = %v, published = %v", err, called)
	}
	if ReadPid(dir) != os.Getppid() {
		t.Fatal("bind failure replaced live owner")
	}
}

func TestClaimStateRejectsSecondStarter(t *testing.T) {
	dir := t.TempDir()
	if err := ClaimState(dir, ProxyState{PID: os.Getpid(), HTTPPort: 1234}); err != nil {
		t.Fatal(err)
	}
	if err := ClaimState(dir, ProxyState{PID: os.Getpid(), HTTPPort: 5678}); err == nil {
		t.Fatal("second claim succeeded")
	}
	if ReadState(dir).HTTPPort != 1234 {
		t.Fatal("second claim replaced state")
	}
}

func TestRunReadyFailureReleasesHTTPListener(t *testing.T) {
	srv := New(Options{RoutesFile: filepath.Join(t.TempDir(), "routes.json")})
	srv.httpAddr = "127.0.0.1:0"
	var addr string
	want := errors.New("publish failed")
	err := srv.RunContext(context.Background(), func() error { addr = srv.httpServer.Addr; return want })
	if !errors.Is(err, want) {
		t.Fatalf("got %v", err)
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		t.Fatalf("listener leaked: %v", err)
	}
	ln.Close()
}
