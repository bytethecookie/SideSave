package presets

import (
	"os"
	"path/filepath"
	"testing"
)

// TestDedupePaths_CollapsesSymlinkedRoot mirrors the real bug: SteamOS
// symlinks ~/.steam/steam to ~/.local/share/Steam, so a scan that treats
// them as two different libraries reports every installed game twice.
func TestDedupePaths_CollapsesSymlinkedRoot(t *testing.T) {
	root := t.TempDir()
	real := filepath.Join(root, "local-share-steam")
	if err := os.Mkdir(real, 0o777); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "steam-steam")
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}

	got := dedupePaths([]string{real, link})
	if len(got) != 1 {
		t.Fatalf("dedupePaths(%v) = %v, want exactly one entry (they're the same directory)", []string{real, link}, got)
	}
	if got[0] != real {
		t.Errorf("got[0] = %q, want %q (the real path, since it was listed first)", got[0], real)
	}
}

func TestDedupePaths_KeepsDistinctRealDirs(t *testing.T) {
	root := t.TempDir()
	a := filepath.Join(root, "a")
	b := filepath.Join(root, "b")
	for _, d := range []string{a, b} {
		if err := os.Mkdir(d, 0o777); err != nil {
			t.Fatal(err)
		}
	}

	got := dedupePaths([]string{a, b})
	if len(got) != 2 {
		t.Fatalf("dedupePaths(%v) = %v, want both distinct directories kept", []string{a, b}, got)
	}
}

// A nonexistent candidate (e.g. a hardcoded Windows default probed on
// Linux) can't be resolved via EvalSymlinks — it must still dedupe on its
// literal path rather than being dropped or panicking.
func TestDedupePaths_NonexistentPathStillDedupes(t *testing.T) {
	root := t.TempDir()
	missing := filepath.Join(root, "does-not-exist")

	got := dedupePaths([]string{missing, missing})
	if len(got) != 1 || got[0] != missing {
		t.Errorf("dedupePaths([missing, missing]) = %v, want [%q]", got, missing)
	}
}
