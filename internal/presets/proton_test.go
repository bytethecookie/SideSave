package presets

import (
	"os"
	"path/filepath"
	"testing"
)

func mustMkFile(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o777); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("x"), 0o666); err != nil {
		t.Fatal(err)
	}
}

// TestScan_ExcludesRealSteamGamesIncludesShortcuts pins the core behavior
// of this fork: Scan() must offer a save only when its compatdata AppID
// resolves via shortcuts.vdf, and must never offer one that resolves via
// a real appmanifest — that's a Steam-Cloud game, already synced by Steam
// itself. An AppID that resolves to neither isn't in scope either.
func TestScan_ExcludesRealSteamGamesIncludesShortcuts(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", "")
	t.Setenv("XDG_CONFIG_HOME", "")
	home := t.TempDir()
	lib := filepath.Join(home, ".local", "share", "Steam")

	// A real Steam game (appmanifest-named) — must be excluded.
	realPrefix := filepath.Join(lib, "steamapps", "compatdata", "374320", "pfx", "drive_c", "users", "steamuser")
	mustMkFile(t, filepath.Join(realPrefix, "AppData", "Roaming", "DarkSoulsIII", "save.sl2"))
	acf := filepath.Join(lib, "steamapps", "appmanifest_374320.acf")
	if err := os.WriteFile(acf, []byte(`"AppState"{"appid""374320""name""DARK SOULS III""installdir""DARK SOULS III"}`), 0o666); err != nil {
		t.Fatal(err)
	}

	// A non-Steam shortcut (shortcuts.vdf-named) — must be included.
	shortcutAppID := int32(-830671666) // 3464295630 unsigned
	shortcutPrefix := filepath.Join(lib, "steamapps", "compatdata", "3464295630", "pfx", "drive_c", "users", "steamuser")
	mustMkFile(t, filepath.Join(shortcutPrefix, "AppData", "Roaming", "StellarBladeSave", "slot1.sav"))
	raw := fakeShortcutsVDF(t, map[string]struct {
		appID int32
		name  string
	}{"0": {appID: shortcutAppID, name: "Stellar Blade"}})
	userdata := filepath.Join(lib, "userdata")
	mkfile(t, filepath.Join(userdata, "190002642", "config", "shortcuts.vdf"), string(raw))

	// A compatdata folder with neither an appmanifest nor a shortcuts.vdf
	// entry — not a real Steam game, not a known shortcut, out of scope.
	mysteryPrefix := filepath.Join(lib, "steamapps", "compatdata", "999999", "pfx", "drive_c", "users", "steamuser")
	mustMkFile(t, filepath.Join(mysteryPrefix, "AppData", "Local", "MysteryGame", "slot0"))

	// Vendor junk inside the shortcut's own prefix — must still be skipped.
	mustMkFile(t, filepath.Join(shortcutPrefix, "AppData", "Roaming", "Microsoft", "stuff.dat"))

	sc := &Scanner{GOOS: "linux", HomeDir: home, SteamRoots: []string{lib}}
	got := sc.Scan(nil)

	var dsFound, shortcutFound, mysteryFound, vendorLeak bool
	for _, d := range got {
		switch d.SavePath {
		case filepath.Join(realPrefix, "AppData", "Roaming", "DarkSoulsIII"):
			dsFound = true
		case filepath.Join(shortcutPrefix, "AppData", "Roaming", "StellarBladeSave"):
			shortcutFound = true
			if d.Name != "Stellar Blade (StellarBladeSave)" {
				t.Errorf("shortcut save Name = %q, want %q", d.Name, "Stellar Blade (StellarBladeSave)")
			}
			if d.AppID != "3464295630" {
				t.Errorf("shortcut save AppID = %q, want 3464295630", d.AppID)
			}
		case filepath.Join(mysteryPrefix, "AppData", "Local", "MysteryGame"):
			mysteryFound = true
		case filepath.Join(shortcutPrefix, "AppData", "Roaming", "Microsoft"):
			vendorLeak = true
		}
	}
	if dsFound {
		t.Error("a real Steam-Cloud game (matched via appmanifest) must be excluded from scan output")
	}
	if !shortcutFound {
		t.Error("a non-Steam shortcut (matched via shortcuts.vdf) must be included")
	}
	if mysteryFound {
		t.Error("a compatdata folder matching neither a real Steam game nor a shortcut must be excluded")
	}
	if vendorLeak {
		t.Error("Wine/Windows vendor junk inside a shortcut's prefix must still be filtered out")
	}
}
