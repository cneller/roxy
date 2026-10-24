package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/logscore/roxy/internal/proxy"
)

func TestStopProxyProcessPreservesReplacement(t *testing.T) {
	dir := t.TempDir()
	oldPID := 1<<22 + 1
	if err := os.WriteFile(proxy.PidFile(dir), []byte(fmt.Sprint(oldPID)), 0600); err != nil {
		t.Fatal(err)
	}
	send := func(pid int, sig syscall.Signal) error {
		if pid != oldPID {
			t.Fatalf("signaled replacement pid %d", pid)
		}
		if sig == syscall.SIGTERM {
			if err := os.WriteFile(proxy.PidFile(dir), []byte(fmt.Sprint(os.Getpid())), 0600); err != nil {
				t.Fatal(err)
			}
			if err := proxy.WriteState(dir, proxy.ProxyState{PID: os.Getpid()}); err != nil {
				t.Fatal(err)
			}
			return nil
		}
		return syscall.ESRCH
	}
	if err := stopProxyProcess(dir, oldPID, send, func(time.Duration) {}); err != nil {
		t.Fatal(err)
	}
	if proxy.ReadPid(dir) != os.Getpid() || proxy.ReadState(dir) == nil {
		t.Fatal("replacement metadata removed")
	}
}

func TestStopProxyProcessRetainsMetadataUntilExit(t *testing.T) {
	for _, probeErr := range []error{nil, syscall.EPERM} {
		t.Run(fmt.Sprint(probeErr), func(t *testing.T) {
			dir := t.TempDir()
			if err := proxy.WritePidFile(dir); err != nil {
				t.Fatal(err)
			}
			send := func(_ int, sig syscall.Signal) error {
				if sig == syscall.SIGTERM {
					return nil
				}
				return probeErr
			}
			if err := stopProxyProcess(dir, os.Getpid(), send, func(time.Duration) {}); err == nil {
				t.Fatal("reported stopped without proof of exit")
			}
			if proxy.ReadPid(dir) != os.Getpid() {
				t.Fatal("removed metadata without proof of exit")
			}
		})
	}
}

func TestStopProxyProcessCleansCapturedDeadOwner(t *testing.T) {
	dir := t.TempDir()
	pid := 1<<22 + 1
	if err := os.WriteFile(proxy.PidFile(dir), []byte(fmt.Sprint(pid)), 0600); err != nil {
		t.Fatal(err)
	}
	if err := proxy.WriteState(dir, proxy.ProxyState{PID: pid}); err != nil {
		t.Fatal(err)
	}
	if err := stopProxyProcess(dir, pid, func(int, syscall.Signal) error { return syscall.ESRCH }, func(time.Duration) {}); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{proxy.PidFile(dir), filepath.Join(dir, "proxy.state.json")} {
		if _, err := os.Stat(name); !os.IsNotExist(err) {
			t.Fatalf("stale file retained: %v", err)
		}
	}
}
