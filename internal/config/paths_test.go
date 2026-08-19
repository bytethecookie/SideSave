package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolve_FreshInstall_CreatesHomeAndBackupsDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	paths, err := Resolve()
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	wantHome := filepath.Join(home, homeDirName)
	if paths.HomeDir != wantHome {
		t.Errorf("HomeDir = %q, want %q", paths.HomeDir, wantHome)
	}
	if _, err := os.Stat(paths.HomeDir); err != nil {
		t.Errorf("home dir not created: %v", err)
	}
	if _, err := os.Stat(paths.BackupsDir); err != nil {
		t.Errorf("backups dir not created: %v", err)
	}
}

func TestResolve_MigratesLegacySavesyncDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	oldHome := filepath.Join(home, savesyncHomeDirName)
	if err := os.MkdirAll(oldHome, 0o777); err != nil {
		t.Fatal(err)
	}
	oldDBPath := filepath.Join(oldHome, "savesync-db.json")
	if err := os.WriteFile(oldDBPath, []byte(`{"settings":{}}`), 0o666); err != nil {
		t.Fatal(err)
	}

	paths, err := Resolve()
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	if _, err := os.Stat(oldHome); !os.IsNotExist(err) {
		t.Errorf("old home dir %q should no longer exist, stat err = %v", oldHome, err)
	}
	if _, err := os.Stat(paths.LegacyDB); err != nil {
		t.Errorf("expected migrated legacy db at %q: %v", paths.LegacyDB, err)
	}
}

func TestResolve_DoesNotOverwriteExistingNewHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	oldHome := filepath.Join(home, savesyncHomeDirName)
	newHome := filepath.Join(home, homeDirName)
	if err := os.MkdirAll(oldHome, 0o777); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(newHome, 0o777); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(newHome, "marker.txt")
	if err := os.WriteFile(marker, []byte("keep me"), 0o666); err != nil {
		t.Fatal(err)
	}

	if _, err := Resolve(); err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	if _, err := os.Stat(marker); err != nil {
		t.Errorf("existing new home dir contents should be preserved: %v", err)
	}
	if _, err := os.Stat(oldHome); err != nil {
		t.Errorf("old home dir should be left alone when new home already exists: %v", err)
	}
}

// TestResolve_MigratesOpenSaveHomeToSideSave covers the case that matters
// for every real install of this fork: no ~/.savesync in sight (that
// migration finished years ago), just a real ~/.opensave from before the
// rename, holding real tracked-game data that must not be orphaned — and
// its database files must be renamed to their new names too, not just the
// directory around them.
func TestResolve_MigratesOpenSaveHomeToSideSave(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	opensaveHome := filepath.Join(home, opensaveHomeDirName)
	if err := os.MkdirAll(opensaveHome, 0o777); err != nil {
		t.Fatal(err)
	}
	dbPath := filepath.Join(opensaveHome, "opensave.db")
	if err := os.WriteFile(dbPath, []byte("real tracked-game data"), 0o666); err != nil {
		t.Fatal(err)
	}

	paths, err := Resolve()
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	if _, err := os.Stat(opensaveHome); !os.IsNotExist(err) {
		t.Errorf("old ~/.opensave should no longer exist after migrating, stat err = %v", err)
	}
	got, err := os.ReadFile(paths.SQLiteDB)
	if err != nil {
		t.Fatalf("expected the database to have moved to %q: %v", paths.SQLiteDB, err)
	}
	if string(got) != "real tracked-game data" {
		t.Errorf("database content = %q, want the original data preserved", got)
	}
}

// A device already fully migrated to ~/.sidesave must never be touched by
// migration logic again, even if a stray ~/.opensave sits beside it (e.g.
// left over from before the first migration ran, or from another install).
func TestResolve_LeavesFullyMigratedHomeAlone(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	sidesaveHome := filepath.Join(home, homeDirName)
	if err := os.MkdirAll(sidesaveHome, 0o777); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(sidesaveHome, "marker.txt")
	if err := os.WriteFile(marker, []byte("keep me"), 0o666); err != nil {
		t.Fatal(err)
	}
	staleOpensaveHome := filepath.Join(home, opensaveHomeDirName)
	if err := os.MkdirAll(staleOpensaveHome, 0o777); err != nil {
		t.Fatal(err)
	}

	if _, err := Resolve(); err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	if _, err := os.Stat(marker); err != nil {
		t.Errorf("existing ~/.sidesave contents should be preserved: %v", err)
	}
	if _, err := os.Stat(staleOpensaveHome); err != nil {
		t.Errorf("a stray ~/.opensave should be left alone once ~/.sidesave already exists: %v", err)
	}
}
