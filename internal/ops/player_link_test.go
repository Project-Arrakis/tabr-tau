package ops

import (
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// ops.player() must follow the game's own link from player_state to the pawn, not guess among the account's actors
// (issue #33). A second PlayerCharacter actor of the same account with a lower id must not be chosen.
func TestPlayerPawnFollowsTheGamesOwnLink(t *testing.T) {
	const twoPawns = `insert into actors(id, class, map, location_x, location_y, location_z, owner_account_id, dimension_index) values
		(4, '/Game/Dune/Characters/Player/BP_DunePlayerCharacter.BP_DunePlayerCharacter_C', 'HaggaBasin', 0, 0, 0, 1, 0);
		update player_state set player_pawn_id=4 where id=1;`
	o := &Ops{S: testsave.PlayerWithSQL(t, twoPawns)}
	p, err := o.player()
	if err != nil {
		t.Fatal(err)
	}
	if p.Pawn != 4 {
		t.Errorf("the pawn is %d; the game links player_state to actor 4", p.Pawn)
	}
	if p.Controller != 1 {
		t.Errorf("the controller is %d, want 1", p.Controller)
	}
}

// A save whose player_state has no pawn link still finds the account's character, lowest id first.
func TestPlayerPawnFallsBackToTheAccountsCharacterWhenThereIsNoLink(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, `update player_state set player_pawn_id=null where id=1;`)}
	p, err := o.player()
	if err != nil {
		t.Fatal(err)
	}
	if p.Pawn != 2 {
		t.Errorf("the fallback pawn is %d, want 2", p.Pawn)
	}
}
