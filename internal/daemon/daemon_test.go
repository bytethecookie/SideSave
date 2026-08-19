package daemon

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/bytethecookie/sidesave/internal/store"
)

func TestAppIDFromCompatdataPath(t *testing.T) {
	cases := []struct {
		path string
		want string
	}{
		{`/home/deck/.local/share/Steam/steamapps/compatdata/2459494291/pfx/drive_c/users/steamuser/AppData/Local/SB`, "2459494291"},
		{`C:\Program Files (x86)\Steam\steamapps\compatdata\2459494291\pfx\drive_c\users\steamuser`, "2459494291"},
		{`/home/deck/.local/share/Steam/userdata/190002642/1426210`, ""}, // not a compatdata path
		{"", ""},
	}
	for _, c := range cases {
		if got := AppIDFromCompatdataPath(c.path); got != c.want {
			t.Errorf("AppIDFromCompatdataPath(%q) = %q, want %q", c.path, got, c.want)
		}
	}
}

func TestIsSteamCover(t *testing.T) {
	if !IsSteamCover("https://cdn.cloudflare.steamstatic.com/steam/apps/570/header.jpg") {
		t.Error("a steamstatic.com cover should be recognized as one")
	}
	if IsSteamCover("https://example.com/my-custom-cover.jpg") {
		t.Error("a custom cover must not be mistaken for a Steam one")
	}
	if IsSteamCover("") {
		t.Error("an empty string is not a Steam cover")
	}
}

// TestValidateSavePath_RejectsSymlinkedDuplicate pins the actual bug: Steam's
// own ~/.steam/steam -> ~/.local/share/Steam convention (and equivalents
// elsewhere) means two different path spellings can be the exact same
// files on disk. A literal string comparison misses that — caught live,
// tracking a scan result reached through the symlink created a second,
// fully duplicate copy of a folder already tracked through the real path,
// with its own separate (and immediately stale) snapshot history.
func TestValidateSavePath_RejectsSymlinkedDuplicate(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "home")
	if err := os.MkdirAll(home, 0o777); err != nil {
		t.Fatal(err)
	}
	d, err := New(Options{HomeOverride: home, DisableDiscovery: true})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(d.Stop)

	real := filepath.Join(root, "real-save-dir")
	if err := os.MkdirAll(real, 0o777); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "symlinked-save-dir")
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}

	if _, err := d.TrackGame(store.Game{Name: "Stellar Blade", SavePath: real}); err != nil {
		t.Fatalf("track via real path: %v", err)
	}

	if _, err := d.ValidateSavePath(link); err == nil {
		t.Error("tracking the same directory through a symlink must be rejected as an existing duplicate, not silently allowed")
	}
}
