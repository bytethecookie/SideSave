package p2p

import (
	"path/filepath"
	"testing"

	"github.com/bytethecookie/sidesave/internal/store"
)

// TestBackfillCover_PrefersLocalPathAppIDOverPeers pins the actual bug: a
// non-Steam shortcut's AppID is a CRC of its own exe path, so it's
// different on every device. A game tracked here at a real Proton prefix
// has its own correct AppID sitting right there in the path — a peer
// backfilling a blank cover must never overwrite that with its own
// device's different id, or the two devices fight over whose id "wins"
// every reconcile.
func TestBackfillCover_PrefersLocalPathAppIDOverPeers(t *testing.T) {
	e, _ := newMatchTestEngine(t)

	local := store.Game{
		ID:   "sb",
		Name: "Stellar Blade",
		SavePath: filepath.Join("home", "deck", ".local", "share", "Steam", "steamapps",
			"compatdata", "2459494291", "pfx", "drive_c", "users", "steamuser", "AppData", "Local", "SB"),
		// CoverURL empty and AppID empty simulate the game right after its
		// path was corrected — set path clears the id under the old bug,
		// or here it's simply not yet backfilled.
	}

	got := e.backfillCover(local, manifestGameQuery{
		AppID:    "2194695776", // the PEER's own (different) device-local id
		CoverURL: "https://cdn.cloudflare.steamstatic.com/steam/apps/2194695776/header.jpg",
	})

	if got.AppID != "2459494291" {
		t.Errorf("AppID = %q, want 2459494291 (derived from this device's own save path, not the peer's 2194695776)", got.AppID)
	}
}

// A game with no Proton-shaped path to derive an id from (an emulator save,
// say) has no local signal at all — the peer's AppID is the only thing
// available, and backfilling it is the whole point of this function.
func TestBackfillCover_FallsBackToPeerAppIDWithNoLocalPath(t *testing.T) {
	e, _ := newMatchTestEngine(t)

	local := store.Game{
		ID:       "retroarch-saves",
		Name:     "Some Emulator Game",
		SavePath: filepath.Join("home", "deck", ".config", "retroarch", "saves"),
	}

	got := e.backfillCover(local, manifestGameQuery{
		AppID:    "123456",
		CoverURL: "https://example.com/cover.jpg",
	})

	if got.AppID != "123456" {
		t.Errorf("AppID = %q, want 123456 (the peer's — nothing local to prefer)", got.AppID)
	}
	if got.CoverURL != "https://example.com/cover.jpg" {
		t.Errorf("CoverURL = %q, want the peer's cover", got.CoverURL)
	}
}
