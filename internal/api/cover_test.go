package api

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLocalGridArt_FindsLandscapeAndPortrait(t *testing.T) {
	userdata := t.TempDir()
	grid := filepath.Join(userdata, "190002642", "config", "grid")
	if err := os.MkdirAll(grid, 0o777); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(grid, "2194695776.jpg"), []byte("landscape"), 0o666); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(grid, "2194695776p.png"), []byte("portrait"), 0o666); err != nil {
		t.Fatal(err)
	}

	orig := steamUserdataPathsFn
	steamUserdataPathsFn = func() []string { return []string{userdata} }
	t.Cleanup(func() { steamUserdataPathsFn = orig })

	data, ext, ok := localGridArt("2194695776", false)
	if !ok || string(data) != "landscape" || ext != "jpg" {
		t.Errorf("landscape lookup = (%q, %q, %v), want (\"landscape\", \"jpg\", true)", data, ext, ok)
	}

	data, ext, ok = localGridArt("2194695776", true)
	if !ok || string(data) != "portrait" || ext != "png" {
		t.Errorf("portrait lookup = (%q, %q, %v), want (\"portrait\", \"png\", true)", data, ext, ok)
	}

	if _, _, ok := localGridArt("999999", false); ok {
		t.Error("expected no match for an AppID with no local art")
	}
}
