package ops

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/save"
	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// An acceptance run of the recycle with a Solido on a real save (#1187). It is for the person who has one, and it skips unless the variables name it. It copies the save, makes
// the Solido of a standing base and recycles the base in the copy with this editor, and checks that the rows read back from the written copy are the ones the conversion makes
// from the base as it was. The copy is left where TABR_SOLIDO_REAL_OUT says, so that `tabr-tau diff` can compare it with the save it was made from. The save is never read from the
// repository and nothing of it is kept here.
//
//	TABR_SOLIDO_REAL_BEFORE  path of the save (it is only read)
//	TABR_SOLIDO_REAL_TOTEM   the totem of a standing base of the character's
//	TABR_SOLIDO_REAL_OUT     path of the copy to write (must not exist)
//	TABR_SOLIDO_REAL_STAND   optional, "1": the base is one that the game's tool picked up (a Deep Desert base cannot stand in the save the game made). In the COPY it is put back as
//	                         a standing base first (its actors in the normal state, its backup rows removed, an owner record for the character), so that the whole edit can be run on it
//	TABR_SOLIDO_REAL_TOTEM2  optional: a second standing base. After the first, the device is filled, so the Solido of the second needs a new device: this says what the editor
//	                         does when the loadout has no free slot (a refusal by name that changes nothing)
func TestRecycleWithASolidoOnARealSave(t *testing.T) {
	src, out, tid := os.Getenv("TABR_SOLIDO_REAL_BEFORE"), os.Getenv("TABR_SOLIDO_REAL_OUT"), os.Getenv("TABR_SOLIDO_REAL_TOTEM")
	if src == "" || out == "" || tid == "" {
		t.Skip("no real save named (TABR_SOLIDO_REAL_BEFORE, TABR_SOLIDO_REAL_TOTEM, TABR_SOLIDO_REAL_OUT)")
	}
	totem, err := strconv.ParseInt(tid, 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(out); err == nil {
		t.Fatal("the output file exists; name one that does not")
	}
	if err := os.MkdirAll(filepath.Dir(out), 0o700); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(src)
	if err != nil {
		t.Skipf("the save is not readable here: %v", err)
	}
	g, err := os.Create(out)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(g, f); err != nil {
		t.Fatal(err)
	}
	f.Close()
	g.Close()
	_ = testsave.HostileText // the package's init makes the process check report no game
	s, err := save.Open(out)
	if err != nil {
		t.Fatal(err)
	}
	o := &Ops{S: s}
	counts := func() map[string]int64 {
		tables, err := s.Query(`select name from sqlite_master where type='table' and name not like 'sqlite_%' order by name`)
		if err != nil {
			t.Fatal(err)
		}
		c := map[string]int64{}
		for _, r := range tables {
			name := r["name"].(string)
			n, err := s.One(`select count(*) n from "` + name + `"`)
			if err != nil {
				t.Fatal(err)
			}
			c[name] = n["n"].(int64)
		}
		return c
	}
	if os.Getenv("TABR_SOLIDO_REAL_STAND") == "1" {
		pl, err := o.player()
		if err != nil {
			t.Fatal(err)
		}
		if _, err := s.Mutate("stand the base up in the copy", func(m *save.Mut) error {
			bk, err := m.One(`select id from base_backup_linked_actors where actor_id=?`, totem)
			if err != nil || bk == nil {
				return fmt.Errorf("the base has no backup: %v", err)
			}
			for _, q := range []string{
				`update actors set state=0 where id in (select actor_id from base_backup_linked_actors where id=` + fmt.Sprint(bk["id"]) + `)`,
				`delete from base_backup_linked_actors where id=` + fmt.Sprint(bk["id"]),
				`delete from base_backups where id=` + fmt.Sprint(bk["id"]),
				`delete from permission_actor where actor_id=?1`,
				`insert into permission_actor(actor_id,actor_name,actor_type,access_level,is_child,edited_by_player_id) values (?1,'##Totem_Placeable',3,3,0,0)`,
				`insert into permission_actor_rank(permission_actor_id,player_id,rank) values (?1,` + strconv.FormatInt(pl.Controller, 10) + `,1)`,
			} {
				if _, err := m.Exec(q, totem); err != nil {
					return fmt.Errorf("%s: %w", q, err)
				}
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		if n, _ := s.One(`select count(*) n from base_backup_linked_actors where actor_id=?`, totem); n["n"] != int64(0) {
			t.Fatal("the backup is still there")
		}
	}
	before := counts()

	// what the conversion makes from the base as it is
	ent, actors, buildings := baseParts(t, s, totem)
	srcRows, err := loadSolidoSource(s, totem, ent, actors, buildings)
	if err != nil {
		t.Fatal(err)
	}
	want, err := convertSolido(srcRows)
	if err != nil {
		t.Fatal(err)
	}
	pl, err := o.player()
	if err != nil {
		t.Fatal(err)
	}
	devs, _ := s.Query(`select i.id id, i.stats stats from items i join inventories v on v.id=i.inventory_id where i.template_id=? and v.actor_id=? order by i.id`, solidoDevice, pl.Pawn)
	t.Logf("the character holds %d Solido devices before: %v", len(devs), devs)

	opt := RecycleOptions{Solido: true, AllowNonEmptyReserve: true} // a real save may hold stacks in the Reserve, and the second base is asked about with the first one's in it
	pv, err := o.PreviewBaseRecycleWith(totem, opt)
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if fmt.Sprint(counts()) != fmt.Sprint(before) {
		t.Fatal("the preview changed the save")
	}
	res, err := o.RecycleBaseWith(totem, pv.(map[string]any)["fingerprint"].(string), opt)
	if err != nil {
		t.Fatalf("recycle: %v", err)
	}
	r := res.(map[string]any)
	sol := r["solido"].(map[string]any)
	after := counts()
	bp := sol["blueprint"].(int64)
	for tbl, n := range map[string]int{"building_blueprint_instances": len(want.Instances), "building_blueprint_placeables": len(want.Placeables), "building_blueprint_pentashields": len(want.Pentas)} {
		if after[tbl]-before[tbl] != int64(n) {
			t.Errorf("%s: %d rows more, and the Solido holds %d", tbl, after[tbl]-before[tbl], n)
		}
	}
	if after["building_blueprints"]-before["building_blueprints"] != 1 {
		t.Errorf("building_blueprints: %d rows more", after["building_blueprints"]-before["building_blueprints"])
	}
	// the written rows are the converted ones, read from the file
	is, _ := s.Query(`select count(*) n from building_blueprint_instances where building_blueprint_id=? and hologram=1 and health=0`, bp)
	if is[0]["n"] != int64(len(want.Instances)) {
		t.Errorf("pieces with hologram 1 and health 0: %v", is)
	}
	item, _ := s.One(`select i.inventory_id inv, v.inventory_type ty, v.actor_id own, i.stats stats from items i join inventories v on v.id=i.inventory_id where i.id=?`, sol["item"])
	if !strings.Contains(fmt.Sprint(item["stats"]), fmt.Sprintf(`"PlayerBlueprintId":"!!bbp#%d"`, bp)) {
		t.Errorf("the device's stats: %v", item)
	}
	t.Logf("base %d %q on %v: Solido #%d in item %v (created: %v, in inventory %v of type %v): %v pieces, %v placeables, %v pentashields; left out %v pieces, %v doors",
		totem, r["name"], r["map"], bp, sol["item"], sol["itemCreated"], item["inv"], item["ty"], sol["pieces"], sol["placeables"], sol["pentashields"], sol["piecesLeftOut"], sol["doorsLeftOut"])
	for _, w := range r["summary"].([]string) {
		t.Log(w)
	}

	if t2 := os.Getenv("TABR_SOLIDO_REAL_TOTEM2"); t2 != "" {
		second, err := strconv.ParseInt(t2, 10, 64)
		if err != nil {
			t.Fatal(err)
		}
		c := counts()
		_, err = o.PreviewBaseRecycleWith(second, opt)
		t.Logf("the Solido of the second base %d: %v", second, err)
		if err == nil {
			t.Log("(a device could be made: the loadout has a free slot)")
		} else if !strings.Contains(err.Error(), "Solido") || !strings.Contains(err.Error(), "Nothing was changed") {
			t.Errorf("the refusal does not name the Solido: %v", err)
		}
		if fmt.Sprint(counts()) != fmt.Sprint(c) {
			t.Fatal("the refused question changed the save")
		}
	}
	if _, err := s.CommitForTests(); err != nil {
		t.Fatal(err)
	}
}
