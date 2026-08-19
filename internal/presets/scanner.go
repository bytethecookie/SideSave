package presets

import (
	"os"
	"runtime"
)

// Scanner performs the full auto-detection sweep. Zero value is usable.
type Scanner struct {
	// SteamUserdataPaths overrides the default Steam install locations
	// when non-nil (tests use this to stay hermetic on machines that have
	// a real Steam library).
	SteamUserdataPaths []string
	// SteamRoots overrides Steam install-root detection (registry +
	// well-known paths) when non-nil. Tests only.
	SteamRoots []string
	// GOOS selects the platform whose save conventions are scanned. Empty
	// means runtime.GOOS; tests set it to exercise Linux logic on any host.
	GOOS string
	// HomeDir overrides the user's home directory for Linux path resolution
	// (Proton prefixes). Empty means os.UserHomeDir. Tests only.
	HomeDir string
}

// linuxHome returns the home dir used to resolve Linux save paths.
func (sc *Scanner) linuxHome() string {
	if sc.HomeDir != "" {
		return sc.HomeDir
	}
	home, _ := os.UserHomeDir()
	return home
}

// NewScanner builds a production Scanner.
func NewScanner() *Scanner {
	return &Scanner{}
}

// goos returns the platform to scan for (runtime default unless overridden).
func (sc *Scanner) goos() string {
	if sc.GOOS != "" {
		return sc.GOOS
	}
	return runtime.GOOS
}

// Scan finds every non-Steam shortcut's save location: a Proton prefix
// under steamapps/compatdata/<appid>/ whose AppID resolves to a name in
// Steam's own shortcuts.vdf. customScanPaths is accepted for API
// compatibility with callers built around the old general-purpose scanner,
// but isn't used — a shortcut with no Steam-shortcut identity to resolve
// isn't in scope for this tool; track it directly with `add` instead.
func (sc *Scanner) Scan(customScanPaths []string) []DiscoveredSave {
	libraries := sc.steamLibraryPaths()

	// Real Steam games (appmanifest_*.acf) are Steam-Cloud-covered — build
	// the set so scanProtonCompat can positively exclude their compatdata
	// folders rather than offering them alongside actual shortcuts.
	apps := steamInstalledApps(libraries)
	realSteamAppIDs := make(map[string]bool, len(apps))
	for _, a := range apps {
		realSteamAppIDs[a.AppID] = true
	}

	// Non-Steam shortcuts: shortcuts.vdf is the only source of truth for
	// their names — a shortcut's AppID is a CRC Steam computed from its own
	// exe path, so it means nothing to any Steam Store lookup and nothing
	// outside this device.
	shortcutNames := steamShortcutAppNames(sc.steamUserdataPaths())

	return sc.scanProtonCompat(libraries, map[string]bool{}, shortcutNames, realSteamAppIDs)
}
