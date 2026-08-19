package presets

import (
	"bytes"
	"encoding/binary"
	"path/filepath"
	"testing"
)

// fakeShortcutsVDF encodes a minimal but structurally real shortcuts.vdf:
// a "shortcuts" object holding one entry per (appID, name) pair, each with
// just the two fields this code reads plus one string and one nested empty
// object (tags), matching the shape a real file has around them.
func fakeShortcutsVDF(t *testing.T, entries map[string]struct {
	appID int32
	name  string
}) []byte {
	t.Helper()
	var buf bytes.Buffer
	writeCString := func(s string) {
		buf.WriteString(s)
		buf.WriteByte(0)
	}

	buf.WriteByte(vdfTypeObject)
	writeCString("shortcuts")
	for idx, e := range entries {
		buf.WriteByte(vdfTypeObject)
		writeCString(idx)

		buf.WriteByte(vdfTypeInt32)
		writeCString("appid")
		var b4 [4]byte
		binary.LittleEndian.PutUint32(b4[:], uint32(e.appID))
		buf.Write(b4[:])

		buf.WriteByte(vdfTypeString)
		writeCString("AppName")
		writeCString(e.name)

		buf.WriteByte(vdfTypeString)
		writeCString("Exe")
		writeCString(`"/games/` + e.name + `.exe"`)

		buf.WriteByte(vdfTypeObject)
		writeCString("tags")
		buf.WriteByte(vdfTypeEnd) // close tags

		buf.WriteByte(vdfTypeEnd) // close this entry
	}
	buf.WriteByte(vdfTypeEnd) // close "shortcuts"
	buf.WriteByte(vdfTypeEnd) // close root
	return buf.Bytes()
}

func TestParseShortcutsVDF(t *testing.T) {
	raw := fakeShortcutsVDF(t, map[string]struct {
		appID int32
		name  string
	}{
		// -830671666 as int32 is 3464295630 unsigned — chosen to land past
		// the int32 sign boundary, since that's exactly where a naive
		// signed read would get the compatdata folder name wrong.
		"0": {appID: -830671666, name: "Stellar Blade"},
		"1": {appID: 123456, name: "Small ID Game"},
	})

	got := parseShortcutsVDF(raw)

	if got["3464295630"] != "Stellar Blade" {
		t.Errorf(`got["3464295630"] = %q, want "Stellar Blade" (got: %+v)`, got["3464295630"], got)
	}
	if got["123456"] != "Small ID Game" {
		t.Errorf(`got["123456"] = %q, want "Small ID Game" (got: %+v)`, got["123456"], got)
	}
	if len(got) != 2 {
		t.Errorf("len(got) = %d, want 2: %+v", len(got), got)
	}
}

func TestParseShortcutsVDF_TruncatedFileDoesNotPanic(t *testing.T) {
	full := fakeShortcutsVDF(t, map[string]struct {
		appID int32
		name  string
	}{"0": {appID: 42, name: "Whatever"}})

	for cut := 0; cut <= len(full); cut++ {
		// Must never panic, however the bytes are chopped.
		parseShortcutsVDF(full[:cut])
	}
}

// TestScanProtonCompat_NamesNonSteamShortcut is the real-world case this
// exists for: a non-Steam game added to Steam gets a compatdata prefix
// named after the shortcut's CRC AppID, but no appmanifest.acf — so
// without shortcuts.vdf, scanProtonCompat has no name for it at all.
func TestScanProtonCompat_NamesNonSteamShortcut(t *testing.T) {
	root := t.TempDir()
	userdata := filepath.Join(root, "userdata")
	shortcutAppID := int32(-830671666) // 3464295630 unsigned

	raw := fakeShortcutsVDF(t, map[string]struct {
		appID int32
		name  string
	}{"0": {appID: shortcutAppID, name: "Stellar Blade"}})
	mkfile(t, filepath.Join(userdata, "190002642", "config", "shortcuts.vdf"), string(raw))

	library := filepath.Join(root, "steam")
	compat := filepath.Join(library, "steamapps", "compatdata", "3464295630", "pfx", "drive_c", "users", "steamuser")
	mkfile(t, filepath.Join(compat, "AppData", "Roaming", "StellarBladeSave", "slot1.sav"), "save data")

	sc := &Scanner{SteamUserdataPaths: []string{userdata}}
	appNames := map[string]string{}
	for appID, name := range steamShortcutAppNames(sc.steamUserdataPaths()) {
		appNames[appID] = name
	}
	found := sc.scanProtonCompat([]string{library}, map[string]bool{}, appNames)

	if len(found) != 1 {
		t.Fatalf("expected one discovered save, got %d: %+v", len(found), found)
	}
	want := "Stellar Blade (StellarBladeSave)"
	if found[0].Name != want {
		t.Errorf("Name = %q, want %q — the shortcut's real name from shortcuts.vdf should attribute this prefix", found[0].Name, want)
	}
	if found[0].AppID != "3464295630" {
		t.Errorf("AppID = %q, want 3464295630", found[0].AppID)
	}
}
