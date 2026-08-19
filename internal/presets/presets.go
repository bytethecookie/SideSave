// Package presets auto-detects non-Steam shortcut save locations: games
// added to Steam via "Add a Non-Steam Game," identified by cross-referencing
// each Proton prefix under steamapps/compatdata/ against Steam's own
// shortcuts.vdf. Real Steam-Cloud games are deliberately excluded — Steam
// already syncs those.
package presets

import (
	"os"
	"regexp"
	"strings"
)

// DiscoveredSave is one auto-detected save location, offered to the user
// for tracking.
type DiscoveredSave struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Type     string `json:"type"` // always "game" — kept for API compatibility
	SavePath string `json:"savePath"`
	AppID    string `json:"appId,omitempty"`
}

var idSanitizeRe = regexp.MustCompile(`[^a-z0-9]`)

func sanitizeID(s string) string {
	return idSanitizeRe.ReplaceAllString(strings.ToLower(s), "-")
}

var digitsOnlyRe = regexp.MustCompile(`^\d+$`)

func isAppID(s string) bool {
	return digitsOnlyRe.MatchString(s)
}

func listSubdirs(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var subdirs []string
	for _, e := range entries {
		if e.IsDir() {
			subdirs = append(subdirs, e.Name())
		}
	}
	return subdirs
}

func dirExists(p string) bool {
	info, err := os.Stat(p)
	return err == nil && info.IsDir()
}
