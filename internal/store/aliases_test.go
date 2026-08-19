package store

import (
	"errors"
	"testing"
)

func TestFindGameByAppID(t *testing.T) {
	s := openTestStore(t)
	if err := s.CreateGame(Game{ID: "nevergrave", Name: "NeverGrave", SavePath: `H:\Steam\NeverGrave`, AppID: "2069710"}); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateGame(Game{ID: "other", Name: "Other", SavePath: `H:\Steam\Other`}); err != nil {
		t.Fatal(err)
	}

	got, err := s.FindGameByAppID("2069710")
	if err != nil {
		t.Fatalf("FindGameByAppID error = %v", err)
	}
	if got.ID != "nevergrave" {
		t.Errorf("FindGameByAppID = %q, want nevergrave", got.ID)
	}

	if _, err := s.FindGameByAppID("0000"); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown appid err = %v, want ErrNotFound", err)
	}
	if _, err := s.FindGameByAppID(""); !errors.Is(err, ErrNotFound) {
		t.Errorf("empty appid err = %v, want ErrNotFound", err)
	}
}

// TestFindGameByAppIDAmbiguous pins the data-integrity guard: tracking one
// title at several save locations means several local games share an App ID.
// Matching must refuse to pick one, or a peer's saves could be written into
// the wrong folder and merge two distinct save sets.
func TestFindGameByAppIDAmbiguous(t *testing.T) {
	s := openTestStore(t)
	for _, g := range []Game{
		{ID: "balatro", Name: "Balatro", SavePath: `C:\Steam\Balatro`, AppID: "2379780"},
		{ID: "balatro-2", Name: "Balatro", SavePath: `D:\GSE\Balatro`, AppID: "2379780"},
	} {
		if err := s.CreateGame(g); err != nil {
			t.Fatal(err)
		}
	}

	if _, err := s.FindGameByAppID("2379780"); !errors.Is(err, ErrAmbiguousAppID) {
		t.Errorf("two games sharing an app id should be ambiguous, got %v", err)
	}

	// Once the duplicate is gone the match is unambiguous again.
	if err := s.DeleteGame("balatro-2"); err != nil {
		t.Fatal(err)
	}
	got, err := s.FindGameByAppID("2379780")
	if err != nil {
		t.Fatalf("FindGameByAppID after removing the duplicate = %v", err)
	}
	if got.ID != "balatro" {
		t.Errorf("resolved to %q, want balatro", got.ID)
	}
}

func TestGameAliasCRUDAndCascade(t *testing.T) {
	s := openTestStore(t)
	if err := s.CreateGame(Game{ID: "nevergrave", Name: "NeverGrave", SavePath: `H:\Steam\NeverGrave`}); err != nil {
		t.Fatal(err)
	}

	if err := s.AddGameAlias("nevergrave-portable", "nevergrave"); err != nil {
		t.Fatalf("AddGameAlias error = %v", err)
	}
	// Re-adding the same alias updates rather than errors (upsert).
	if err := s.AddGameAlias("nevergrave-portable", "nevergrave"); err != nil {
		t.Fatalf("re-AddGameAlias error = %v", err)
	}

	if got, ok := s.ResolveGameAlias("nevergrave-portable"); !ok || got != "nevergrave" {
		t.Errorf("ResolveGameAlias = (%q,%v), want (nevergrave,true)", got, ok)
	}
	if _, ok := s.ResolveGameAlias("unknown"); ok {
		t.Errorf("ResolveGameAlias(unknown) should be false")
	}

	aliases, err := s.ListGameAliases("nevergrave")
	if err != nil {
		t.Fatal(err)
	}
	if len(aliases) != 1 || aliases[0] != "nevergrave-portable" {
		t.Errorf("ListGameAliases = %v, want [nevergrave-portable]", aliases)
	}

	// Invalid links are rejected.
	if err := s.AddGameAlias("x", "x"); err == nil {
		t.Error("self-alias should be rejected")
	}
	if err := s.AddGameAlias("", "nevergrave"); err == nil {
		t.Error("empty alias should be rejected")
	}

	// Deleting the canonical game cascades its aliases away.
	if err := s.DeleteGame("nevergrave"); err != nil {
		t.Fatal(err)
	}
	if _, ok := s.ResolveGameAlias("nevergrave-portable"); ok {
		t.Error("alias should be gone after canonical game deleted (ON DELETE CASCADE)")
	}
}

func TestRemoveGameAlias(t *testing.T) {
	s := openTestStore(t)
	if err := s.CreateGame(Game{ID: "a", Name: "A", SavePath: `C:\A`}); err != nil {
		t.Fatal(err)
	}
	if err := s.AddGameAlias("b", "a"); err != nil {
		t.Fatal(err)
	}
	if err := s.RemoveGameAlias("b"); err != nil {
		t.Fatalf("RemoveGameAlias error = %v", err)
	}
	if _, ok := s.ResolveGameAlias("b"); ok {
		t.Error("alias should be gone after RemoveGameAlias")
	}
}

func TestFindPeerPlaceholderByName(t *testing.T) {
	s := openTestStore(t)
	if err := s.CreateGame(Game{
		ID: "sackboy", Name: "Sackboy: A Big Adventure",
		SavePath: `/home/aiboxadmin/.local/share/Steam/steamapps/compatdata/3472464288/pfx/.../Sackboy`,
		PeerPlaceholder: true,
	}); err != nil {
		t.Fatal(err)
	}
	// A locally-scanned game with the same name is NOT a placeholder — it
	// must never shadow the real thing on a lookup.
	if err := s.CreateGame(Game{ID: "other", Name: "Other Game", SavePath: `/home/x/Other`}); err != nil {
		t.Fatal(err)
	}

	got, err := s.FindPeerPlaceholderByName("Sackboy: A Big Adventure")
	if err != nil {
		t.Fatalf("FindPeerPlaceholderByName error = %v", err)
	}
	if got.ID != "sackboy" {
		t.Errorf("FindPeerPlaceholderByName = %q, want sackboy", got.ID)
	}

	if _, err := s.FindPeerPlaceholderByName("Other Game"); !errors.Is(err, ErrNotFound) {
		t.Errorf("non-placeholder game name err = %v, want ErrNotFound", err)
	}
	if _, err := s.FindPeerPlaceholderByName("Nothing Tracked"); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown name err = %v, want ErrNotFound", err)
	}
}

// Once a placeholder's path is explicitly set to the real install, it's no
// longer a placeholder — a second install found later has nothing left to
// warn about, and the same setting is what TrackGame tells the user to run.
func TestUpdateGame_ClearingPlaceholderViaPathChange(t *testing.T) {
	s := openTestStore(t)
	g := Game{ID: "sackboy", Name: "Sackboy: A Big Adventure", SavePath: `/old/path`, PeerPlaceholder: true}
	if err := s.CreateGame(g); err != nil {
		t.Fatal(err)
	}

	g.SavePath = `/real/path`
	g.PeerPlaceholder = false
	if err := s.UpdateGame(g); err != nil {
		t.Fatalf("UpdateGame error = %v", err)
	}

	if _, err := s.FindPeerPlaceholderByName("Sackboy: A Big Adventure"); !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound — the game was just confirmed as real, not a placeholder anymore", err)
	}
}
