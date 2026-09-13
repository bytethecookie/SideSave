package main

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRevealTargetDir covers what "Open folder" resolves to before anything
// is launched: a directory reveals itself, a single-file save reveals its
// parent, and unusable paths come back with a message the user can act on
// rather than a silently dead button.
func TestRevealTargetDir(t *testing.T) {
	base := t.TempDir()

	t.Run("directory reveals itself", func(t *testing.T) {
		dir := filepath.Join(base, "SaveFolder")
		if err := os.MkdirAll(dir, 0o777); err != nil {
			t.Fatal(err)
		}
		got, problem := revealTargetDir(dir)
		if problem != "" {
			t.Fatalf("unexpected problem: %s", problem)
		}
		if got != filepath.Clean(dir) {
			t.Errorf("got %q, want %q", got, filepath.Clean(dir))
		}
	})

	t.Run("file reveals its parent", func(t *testing.T) {
		dir := filepath.Join(base, "SingleFileGame")
		if err := os.MkdirAll(dir, 0o777); err != nil {
			t.Fatal(err)
		}
		file := filepath.Join(dir, "slot1.sav")
		if err := os.WriteFile(file, []byte("save"), 0o666); err != nil {
			t.Fatal(err)
		}
		got, problem := revealTargetDir(file)
		if problem != "" {
			t.Fatalf("unexpected problem: %s", problem)
		}
		if got != filepath.Clean(dir) {
			t.Errorf("got %q, want the containing folder %q", got, filepath.Clean(dir))
		}
	})

	t.Run("empty path is reported", func(t *testing.T) {
		if _, problem := revealTargetDir(""); problem == "" {
			t.Error("empty path should report a problem")
		}
		if _, problem := revealTargetDir("   "); problem == "" {
			t.Error("whitespace-only path should report a problem")
		}
	})

	t.Run("missing folder is reported", func(t *testing.T) {
		_, problem := revealTargetDir(filepath.Join(base, "not-here"))
		if problem == "" {
			t.Fatal("missing folder should report a problem")
		}
		if !strings.Contains(strings.ToLower(problem), "no longer exists") {
			t.Errorf("message should say the folder is gone, got %q", problem)
		}
	})
}

// TestRunningDaemonAddr pins the guard added after two independent
// daemons — the app's own embedded one and a separately-running
// sidesave-cli — ended up racing against the same data dir, each
// mistaking the other's writes for local changes and looping on sync
// conflicts (caught live on aibox). startup() must detect an
// already-answering daemon via daemon.addr before starting a competing
// one, the same reachability check internal/cliapp's daemonRunning
// already used for `sidesave daemon start`.
func TestRunningDaemonAddr(t *testing.T) {
	t.Run("no addr file at all", func(t *testing.T) {
		if got := runningDaemonAddr(t.TempDir()); got != "" {
			t.Errorf("got %q, want \"\" — nothing published an address", got)
		}
	})

	t.Run("addr file points at nothing listening", func(t *testing.T) {
		home := t.TempDir()
		// Grab a port and immediately release it — nothing answers there.
		l, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		stale := l.Addr().String()
		l.Close()
		if err := os.WriteFile(filepath.Join(home, "daemon.addr"), []byte(stale), 0o666); err != nil {
			t.Fatal(err)
		}
		if got := runningDaemonAddr(home); got != "" {
			t.Errorf("got %q, want \"\" — a stale addr file must not be treated as a live daemon", got)
		}
	})

	t.Run("addr file points at a real daemon", func(t *testing.T) {
		home := t.TempDir()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/api/status" {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()
		addr := srv.Listener.Addr().String()
		if err := os.WriteFile(filepath.Join(home, "daemon.addr"), []byte(fmt.Sprintf("%s\n", addr)), 0o666); err != nil {
			t.Fatal(err)
		}
		got := runningDaemonAddr(home)
		if got != addr {
			t.Errorf("got %q, want %q — a real answering daemon must be detected", got, addr)
		}
	})
}
