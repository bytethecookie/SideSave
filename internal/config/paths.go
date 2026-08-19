// Package config resolves SideSave's on-disk locations and performs the
// one-time home-directory migration chain: ".savesync" -> ".opensave" (the
// original JS app's own migration) -> ".sidesave" (this fork's rename).
package config

import (
	"fmt"
	"os"
	"path/filepath"
)

const (
	savesyncHomeDirName = ".savesync"
	opensaveHomeDirName = ".opensave"
	homeDirName         = ".sidesave"

	// LegacyDBFileName is the JSON database file written by the original
	// Node.js daemon (src/daemon/db.js).
	LegacyDBFileName = "sidesave-db.json"

	// SQLiteFileName is the embedded database file.
	SQLiteFileName = "sidesave.db"

	backupsDirName = "backups"
)

// Paths holds every filesystem location SideSave needs, resolved once at
// startup relative to the user's home directory.
type Paths struct {
	HomeDir      string
	LegacyDB     string
	SQLiteDB     string
	BackupsDir   string
	MigrationLog string
}

// Resolve migrates ~/.savesync -> ~/.opensave -> ~/.sidesave as needed
// (each step only if the old name exists and the new one doesn't, so a
// fresh install or one already on ~/.sidesave does nothing), ensures the
// home directory exists, and returns the resolved Paths.
func Resolve() (Paths, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Paths{}, fmt.Errorf("resolve home dir: %w", err)
	}
	return resolveWithin(home)
}

// ResolveAt uses dataDir directly as the SideSave home directory (tests
// and portable installs) — no migration is attempted.
func ResolveAt(dataDir string) (Paths, error) {
	return buildPaths(dataDir)
}

func resolveWithin(home string) (Paths, error) {
	savesyncHome := filepath.Join(home, savesyncHomeDirName)
	opensaveHome := filepath.Join(home, opensaveHomeDirName)
	sidesaveHome := filepath.Join(home, homeDirName)

	// Already fully migrated (or a fresh install that's never used either
	// old name) — nothing to do, and any stray old-named directory left
	// beside it is none of this function's business.
	if dirExists(sidesaveHome) {
		return buildPaths(sidesaveHome)
	}

	if dirExists(savesyncHome) && !dirExists(opensaveHome) {
		if err := os.Rename(savesyncHome, opensaveHome); err != nil {
			return Paths{}, fmt.Errorf("migrate %s to %s: %w", savesyncHome, opensaveHome, err)
		}
		// The old JS app also renamed a stray savesync-db.json inside the
		// folder if the outer rename above hadn't already moved it.
		oldDBFile := filepath.Join(opensaveHome, "savesync-db.json")
		newDBFile := filepath.Join(opensaveHome, "opensave-db.json")
		if fileExists(oldDBFile) && !fileExists(newDBFile) {
			_ = os.Rename(oldDBFile, newDBFile)
		}
	}

	if dirExists(opensaveHome) {
		if err := migrateOpenSaveContents(opensaveHome, sidesaveHome); err != nil {
			return Paths{}, err
		}
	}

	return buildPaths(sidesaveHome)
}

// migrateOpenSaveContents moves an ~/.opensave directory to ~/.sidesave,
// renaming the database files it holds to their new names on the way —
// the directory rename alone would leave a ~/.sidesave full of
// opensave.db/opensave-db.json, which nothing in this fork looks for.
func migrateOpenSaveContents(opensaveHome, sidesaveHome string) error {
	if err := os.Rename(opensaveHome, sidesaveHome); err != nil {
		return fmt.Errorf("migrate %s to %s: %w", opensaveHome, sidesaveHome, err)
	}
	renames := map[string]string{
		"opensave.db":      SQLiteFileName,
		"opensave-db.json": LegacyDBFileName,
	}
	for old, new := range renames {
		oldPath := filepath.Join(sidesaveHome, old)
		newPath := filepath.Join(sidesaveHome, new)
		if fileExists(oldPath) && !fileExists(newPath) {
			_ = os.Rename(oldPath, newPath)
		}
	}
	return nil
}

func buildPaths(homeDir string) (Paths, error) {
	if err := os.MkdirAll(homeDir, 0o777); err != nil {
		return Paths{}, fmt.Errorf("create home dir: %w", err)
	}
	backupsDir := filepath.Join(homeDir, backupsDirName)
	if err := os.MkdirAll(backupsDir, 0o777); err != nil {
		return Paths{}, fmt.Errorf("create backups dir: %w", err)
	}

	return Paths{
		HomeDir:      homeDir,
		LegacyDB:     filepath.Join(homeDir, LegacyDBFileName),
		SQLiteDB:     filepath.Join(homeDir, SQLiteFileName),
		BackupsDir:   backupsDir,
		MigrationLog: filepath.Join(homeDir, "migration.log"),
	}, nil
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
