package presets

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Proton/Wine save detection for Linux (Steam Deck and desktop).
//
// Windows games run through Proton write their saves inside a per-game Wine
// prefix at <library>/steamapps/compatdata/<appid>/pfx/drive_c/users/
// steamuser/. Each prefix belongs to exactly one Steam AppID, so anything
// found inside is attributed to that game (and picks up its cover art).

// protonSaveRoots are the Windows save conventions, relative to a prefix's
// steamuser home, that hold game data worth offering.
var protonSaveRoots = []string{
	filepath.Join("AppData", "Roaming"),
	filepath.Join("AppData", "Local"),
	filepath.Join("AppData", "LocalLow"),
	filepath.Join("Documents", "My Games"),
	filepath.Join("Documents"),
	filepath.Join("Saved Games"),
}

// protonVendorSkip are folders inside a prefix that are Windows/Wine
// plumbing or ubiquitous middleware, never a game's own save.
var protonVendorSkip = map[string]bool{
	"microsoft": true, "windows": true, "wine": true, "temp": true,
	"packages": true, "programs": true, "connecteddevicesplatform": true,
	"comms": true, "d3dscache": true, "inputmethod": true, "vulkan": true,
	"crashdumps": true, "diagnostics": true, "history": true,
	// GPU vendors, middleware, and launcher plumbing that show up in busy
	// prefixes — never a game's own save.
	"nvidia": true, "nvidia corporation": true, "amd": true, "intel": true,
	"criware": true, "unity": true, "unitycrashhandler": true,
	"easyanticheat": true, "battleye": true, "steam": true, "valve": true,
	"epicgameslauncher": true, "goginstaller": true,
}

// scanProtonCompat walks every Steam library's compatdata prefixes and
// offers the save locations of non-Steam shortcuts specifically:
// realSteamAppIDs excludes real Steam-Cloud games (already synced by
// Steam itself), and shortcutNames — resolved from shortcuts.vdf — is
// required for what's left, since an id with no shortcut entry is neither
// a real Steam game nor a known shortcut and isn't in scope here.
func (sc *Scanner) scanProtonCompat(libraries []string, seen map[string]bool, shortcutNames map[string]string, realSteamAppIDs map[string]bool) []DiscoveredSave {
	var found []DiscoveredSave

	for _, lib := range libraries {
		compat := filepath.Join(lib, "steamapps", "compatdata")
		entries, err := os.ReadDir(compat)
		if err != nil {
			continue
		}
		for _, e := range entries {
			appID := e.Name()
			if !e.IsDir() || !isAppID(appID) || realSteamAppIDs[appID] {
				continue
			}
			gameName := shortcutNames[appID]
			if gameName == "" {
				continue
			}
			// Steam's own prefixes use "steamuser", but a prefix created by
			// another tool and later added to Steam can carry the real
			// account name — take whichever users exist.
			userHomes := prefixUserDirs(filepath.Join(compat, appID, "pfx"))
			if len(userHomes) == 0 {
				continue
			}
			steamUser := userHomes[0]

			perGame := 0
			for _, root := range protonSaveRoots {
				rootPath := filepath.Join(steamUser, root)
				for _, sub := range listSubdirs(rootPath) {
					if protonVendorSkip[toLowerASCII(sub)] || looksLikeHexHash(sub) {
						continue
					}
					savePath := filepath.Join(rootPath, sub)
					abs, err := filepath.Abs(savePath)
					if err != nil || seen[abs] || !dirNonEmpty(abs) {
						continue
					}
					// The Ludusavi pass already found a precise save inside
					// this folder — the broad parent would be junk on top.
					if seenInside(seen, abs) {
						continue
					}
					seen[abs] = true

					found = append(found, DiscoveredSave{
						ID:       "proton-" + appID + "-" + sanitizeID(root) + "-" + sanitizeID(sub),
						Name:     fmt.Sprintf("%s (%s)", gameName, sub),
						Type:     "game",
						SavePath: savePath,
						AppID:    appID,
					})
					perGame++
					if perGame >= 12 { // a single prefix shouldn't flood the grid
						break
					}
				}
				if perGame >= 12 {
					break
				}
			}
		}
	}
	return found
}

// prefixUserDirs returns the per-user home directories inside a prefix.
// Steam's own prefixes always use "steamuser", but a prefix created by
// another tool and later added to Steam can carry the real account name.
func prefixUserDirs(prefix string) []string {
	usersDir := filepath.Join(prefix, "drive_c", "users")
	var out []string
	for _, user := range listSubdirs(usersDir) {
		if user == "Public" || user == "Default" || user == "Default User" || user == "All Users" {
			continue
		}
		out = append(out, filepath.Join(usersDir, user))
	}
	return out
}

// seenInside reports whether an already-discovered save lives inside dir.
func seenInside(seen map[string]bool, dir string) bool {
	prefix := toLowerASCII(dir) + string(filepath.Separator)
	for p := range seen {
		if strings.HasPrefix(toLowerASCII(p), prefix) {
			return true
		}
	}
	return false
}

func toLowerASCII(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + 32
		}
	}
	return string(b)
}

// looksLikeHexHash reports cache dirs named as long hex digests (browser /
// shader caches that sometimes land in a Proton prefix).
func looksLikeHexHash(s string) bool {
	if len(s) < 32 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') && (c < 'A' || c > 'F') {
			return false
		}
	}
	return true
}
