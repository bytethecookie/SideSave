package daemon

import "testing"

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
