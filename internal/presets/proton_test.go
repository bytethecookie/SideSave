package presets

import (
	"os"
	"path/filepath"
	"strings"
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
			// GameName must be the shortcut's name on its own, with no
			// "(subfolder)" suffix — the frontend's placeholder-relink
			// detection matches on this against a tracked game's own name,
			// which never carries that suffix either. This regressed once
			// already: matching against Name instead meant a scan result
			// for a game with more than one save location could never
			// match its own tracked placeholder.
			if d.GameName != "Stellar Blade" {
				t.Errorf("shortcut save GameName = %q, want %q (no subfolder suffix)", d.GameName, "Stellar Blade")
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

// TestScan_UnrealSavedFolderOffersOnlySaveGames pins the fix for tracking
// device-specific settings and crash reports as if they were save data.
// A game that follows Unreal's own <GameFolder>/Saved/{SaveGames,Config,
// Logs} convention must only offer SaveGames — Config holds
// GameUserSettings.ini (resolution/graphics), which has no business
// following a save between a Steam Deck and a 4K HDR desktop, and Logs is
// just log files. The engine-wide AppData/Local/UnrealEngine folder (not
// nested under any one game) must be excluded entirely too — it's the
// crash reporter's own cache, shared across every UE game in the prefix.
func TestScan_UnrealSavedFolderOffersOnlySaveGames(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", "")
	t.Setenv("XDG_CONFIG_HOME", "")
	home := t.TempDir()
	lib := filepath.Join(home, ".local", "share", "Steam")

	prefix := filepath.Join(lib, "steamapps", "compatdata", "2194695776", "pfx", "drive_c", "users", "steamuser")

	// The real save data — must be offered.
	mustMkFile(t, filepath.Join(prefix, "AppData", "Local", "SB", "Saved", "SaveGames", "76561197960285355", "StellarBladeSave00.sav"))
	// Device-specific settings — must NOT be offered.
	mustMkFile(t, filepath.Join(prefix, "AppData", "Local", "SB", "Saved", "Config", "WindowsNoEditor", "GameUserSettings.ini"))
	// Crash reports and logs — must NOT be offered.
	mustMkFile(t, filepath.Join(prefix, "AppData", "Local", "SB", "Saved", "Config", "CrashReportClient", "CrashReportClient.ini"))
	mustMkFile(t, filepath.Join(prefix, "AppData", "Local", "SB", "Saved", "Logs", "SB.log"))
	// A flat (non-Unreal) save folder alongside it — must still be offered
	// as before, unaffected by the Saved/ drill-down logic.
	mustMkFile(t, filepath.Join(prefix, "AppData", "Roaming", "GSE Saves", "3489700", "playtime.txt"))
	// The engine-wide crash-reporter cache, a peer of SB (not nested under
	// it) — must be excluded entirely.
	mustMkFile(t, filepath.Join(prefix, "AppData", "Local", "UnrealEngine", "4.26", "Saved", "Something.log"))

	raw := fakeShortcutsVDF(t, map[string]struct {
		appID int32
		name  string
	}{"0": {appID: -2100271520, name: "Stellar Blade"}}) // -2100271520 as int32 is 2194695776 unsigned
	userdata := filepath.Join(lib, "userdata")
	mkfile(t, filepath.Join(userdata, "190002642", "config", "shortcuts.vdf"), string(raw))

	sc := &Scanner{GOOS: "linux", HomeDir: home, SteamRoots: []string{lib}}
	got := sc.Scan(nil)

	var saveGamesFound, gseFound, configLeak, logsLeak, crashLeak, engineLeak bool
	for _, d := range got {
		switch {
		case strings.Contains(d.SavePath, filepath.Join("SB", "Saved", "SaveGames")):
			saveGamesFound = true
			if d.Name != "Stellar Blade (SB)" {
				t.Errorf("SaveGames entry Name = %q, want %q", d.Name, "Stellar Blade (SB)")
			}
		case strings.Contains(d.SavePath, "GSE Saves"):
			gseFound = true
		case strings.Contains(d.SavePath, filepath.Join("Saved", "Config")):
			configLeak = true
		case strings.Contains(d.SavePath, filepath.Join("Saved", "Logs")):
			logsLeak = true
		case strings.Contains(d.SavePath, "CrashReportClient"):
			crashLeak = true
		case strings.Contains(d.SavePath, "UnrealEngine"):
			engineLeak = true
		}
	}
	if !saveGamesFound {
		t.Error("Saved/SaveGames must be offered — it's the actual save data")
	}
	if !gseFound {
		t.Error("a flat (non-Unreal) save folder alongside it must still be offered")
	}
	if configLeak {
		t.Error("Saved/Config must never be offered — it holds device-specific settings, not save data")
	}
	if logsLeak {
		t.Error("Saved/Logs must never be offered — it's just log files")
	}
	if crashLeak {
		t.Error("CrashReportClient must never be offered")
	}
	if engineLeak {
		t.Error("the engine-wide AppData/Local/UnrealEngine folder must never be offered — it's not per-game save data")
	}
}
