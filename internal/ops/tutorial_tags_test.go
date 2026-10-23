package ops

import (
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// Another player's rows sit in the same tables. The tutorial and tag edits must touch the character's own rows only
// (register 2026-10-15, QA-3: TutorialSet and TagSet had no test of their scope).
const otherPlayersRows = `insert into tutorials(id, name) values (7, 'T7');
	insert into tutorial_per_player(player_id, tutorial_id, tutorial_state) values (99, 7, 1);
	insert into player_tags(character_id, tag) values (99, 'Shared.Tag');`

func count(t *testing.T, o *Ops, q string, a ...any) int64 {
	t.Helper()
	return mustOne(t, o, q, a...)["n"].(int64)
}

func TestTutorialSetTouchesOnlyTheCharactersOwnRow(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, otherPlayersRows)}
	if _, err := o.TutorialSet(Args{"id": 7.0}); err != nil {
		t.Fatal(err)
	}
	if n := count(t, o, `select tutorial_state n from tutorial_per_player where player_id=1 and tutorial_id=7`); n != 2 {
		t.Errorf("the character's tutorial is state %d, want 2 (complete)", n)
	}
	if n := count(t, o, `select tutorial_state n from tutorial_per_player where player_id=99 and tutorial_id=7`); n != 1 {
		t.Errorf("another player's tutorial changed to state %d", n)
	}
	if _, err := o.TutorialSet(Args{"id": 7.0, "complete": false}); err != nil {
		t.Fatal(err)
	}
	if n := count(t, o, `select count(*) n from tutorial_per_player where player_id=1`); n != 0 {
		t.Errorf("the character still has %d tutorial rows after un-completing", n)
	}
	if n := count(t, o, `select count(*) n from tutorial_per_player where player_id=99`); n != 1 {
		t.Errorf("another player's tutorial row was removed (%d left)", n)
	}
	for _, id := range []float64{0, -1, 32768} {
		if _, err := o.TutorialSet(Args{"id": id}); err == nil {
			t.Errorf("tutorial id %v must be refused", id)
		}
	}
}

func TestTagSetTouchesOnlyTheCharactersOwnTags(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, otherPlayersRows)}
	for i := 0; i < 2; i++ { // adding twice is not an error and adds one row
		if _, err := o.TagSet(Args{"tag": "Shared.Tag"}); err != nil {
			t.Fatal(err)
		}
	}
	if n := count(t, o, `select count(*) n from player_tags where character_id=1 and tag='Shared.Tag'`); n != 1 {
		t.Errorf("the character has %d copies of the tag, want 1", n)
	}
	if n := count(t, o, `select count(*) n from player_tags where character_id=99`); n != 1 {
		t.Errorf("another player's tags changed (%d)", n)
	}
	if _, err := o.TagSet(Args{"tag": "Shared.Tag", "add": false}); err != nil {
		t.Fatal(err)
	}
	if n := count(t, o, `select count(*) n from player_tags where character_id=1`); n != 0 {
		t.Errorf("the character still has %d tags after removing it", n)
	}
	if n := count(t, o, `select count(*) n from player_tags where character_id=99 and tag='Shared.Tag'`); n != 1 {
		t.Errorf("removing the character's tag removed another player's")
	}
	for _, tag := range []string{"", "bad tag", "x'; drop table items;--", "a/b"} {
		if _, err := o.TagSet(Args{"tag": tag}); err == nil {
			t.Errorf("tag %q must be refused", tag)
		}
	}
}
