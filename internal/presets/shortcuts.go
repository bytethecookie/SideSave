package presets

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"strconv"
)

// Steam's binary VDF ("shortcuts.vdf") lists every non-Steam game the user
// added. Each entry's "appid" field is a CRC32-derived ID the client
// computes once when the shortcut is created — and that same number is
// what Steam later names that game's Proton prefix after, under
// steamapps/compatdata/<appid>/. Reading the field back here means a
// non-Steam prefix can be attributed to its real name with no CRC
// re-implementation and no network lookup: Steam already did the hashing,
// this just reads what it wrote down.
//
// It's a different (undocumented, Valve-internal) binary keyvalues format
// from the plain-text VDF the rest of Steam's config uses, hence its own
// small parser here rather than reusing the *.acf regexes in steamlib.go.

const (
	vdfTypeObject = 0x00
	vdfTypeString = 0x01
	vdfTypeInt32  = 0x02
	vdfTypeEnd    = 0x08
)

// steamShortcutAppNames reads every userdata/<user>/config/shortcuts.vdf
// under the given userdata roots and returns AppID (decimal, matching the
// compatdata folder Steam derives for that shortcut) -> display name.
// Best-effort: a missing or malformed file simply contributes nothing.
func steamShortcutAppNames(userdataPaths []string) map[string]string {
	names := map[string]string{}
	for _, steamPath := range userdataPaths {
		for _, user := range listSubdirs(steamPath) {
			raw, err := os.ReadFile(filepath.Join(steamPath, user, "config", "shortcuts.vdf"))
			if err != nil {
				continue
			}
			for appID, name := range parseShortcutsVDF(raw) {
				names[appID] = name
			}
		}
	}
	return names
}

// parseShortcutsVDF decodes shortcuts.vdf's binary keyvalues and returns
// the AppID -> AppName pairs of its top-level shortcut entries. Any
// structural surprise (a future format change, a truncated file) stops the
// walk and returns whatever was already decoded rather than panicking.
func parseShortcutsVDF(raw []byte) map[string]string {
	names := map[string]string{}
	p := &vdfParser{data: raw}
	root := p.readObject()

	shortcuts, _ := root["shortcuts"].(map[string]any)
	for _, v := range shortcuts {
		entry, ok := v.(map[string]any)
		if !ok {
			continue
		}
		appID, ok1 := entry["appid"].(int32)
		name, ok2 := entry["AppName"].(string)
		if !ok1 || !ok2 || name == "" {
			continue
		}
		// Stored as a signed int32; the compatdata folder name (and every
		// other place Steam prints this ID) is its unsigned decimal form.
		names[strconv.FormatUint(uint64(uint32(appID)), 10)] = name
	}
	return names
}

// vdfParser walks shortcuts.vdf's binary keyvalues left to right. It never
// panics on malformed input — readObject and readCString report failure by
// returning early, and every caller checks before trusting what came back.
type vdfParser struct {
	data   []byte
	pos    int
	failed bool
}

func (p *vdfParser) readObject() map[string]any {
	obj := map[string]any{}
	for {
		if p.failed || p.pos >= len(p.data) {
			p.failed = true
			return obj
		}
		t := p.data[p.pos]
		p.pos++
		if t == vdfTypeEnd {
			return obj
		}
		name, ok := p.readCString()
		if !ok {
			p.failed = true
			return obj
		}
		switch t {
		case vdfTypeObject:
			obj[name] = p.readObject()
		case vdfTypeString:
			s, ok := p.readCString()
			if !ok {
				p.failed = true
				return obj
			}
			obj[name] = s
		case vdfTypeInt32:
			if p.pos+4 > len(p.data) {
				p.failed = true
				return obj
			}
			obj[name] = int32(binary.LittleEndian.Uint32(p.data[p.pos : p.pos+4]))
			p.pos += 4
		default:
			// An unrecognized field type means the bytes after it can't be
			// trusted either — stop instead of misreading the rest of the
			// file as something it's not.
			p.failed = true
			return obj
		}
	}
}

func (p *vdfParser) readCString() (string, bool) {
	start := p.pos
	for p.pos < len(p.data) {
		if p.data[p.pos] == 0 {
			s := string(p.data[start:p.pos])
			p.pos++
			return s, true
		}
		p.pos++
	}
	return "", false
}
