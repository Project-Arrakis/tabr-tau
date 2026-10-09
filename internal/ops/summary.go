package ops

import "strconv"

// maxIntel is the cap the console applies to spendable Intel (MAX_INTEL_POINTS).
const maxIntel = 2779

// baseMaxHealth is the character's maximum health before Vitality bonuses. The save stores only the current health, so the
// maximum is shown as "at least this" and never as a value read from the game (the console also marks it estimated).
const baseMaxHealth = 150

// Summary is the Player Summary header of the console's player view, read from the save: identity, level, XP, skill points,
// Intel, vitals, faction alignment, database ids and the currency balances. Read-only.
func (o *Ops) Summary() (any, error) {
	p, err := o.player()
	if err != nil {
		return nil, err
	}
	out := map[string]any{"name": p.Name, "guild": nil}

	if st, _ := o.S.One(`select online_status, player_state_id from player_state where id=?`, p.Controller); len(st) > 0 {
		out["status"] = "Offline"
		if n, _ := st["online_status"].(int64); n > 0 {
			out["status"] = "Online"
		}
		out["ids"] = map[string]any{"actor": p.Pawn, "account": p.Account, "controller": p.Controller, "playerState": st["player_state_id"]}
	}
	if a, _ := o.S.One(`select map from actors where id=?`, p.Pawn); len(a) > 0 {
		out["map"] = a["map"]
	}
	if acc, _ := o.S.One(`select platform_name, platform_id, funcom_id, "user" fls_id from accounts where id=?`, p.Account); len(acc) > 0 {
		out["identity"] = map[string]any{"platform": acc["platform_name"], "platformId": acc["platform_id"], "funcomId": acc["funcom_id"], "flsId": acc["fls_id"]}
	}

	// The character's own faction row belongs to the controller, not to the pawn.
	if f, _ := o.S.One(`select f.name from player_faction pf join factions f on f.id=pf.faction_id where pf.actor_id=?`, p.Controller); len(f) > 0 {
		out["faction"] = f["name"]
	}

	// Level, XP and skill points live in the character's FLevelComponent.
	lv, _ := o.S.One(`select json_extract(f.components,'$.FLevelComponent[1].TotalXPEarned') xp,
			json_extract(f.components,'$.FLevelComponent[1].TotalSkillPoints') total,
			json_extract(f.components,'$.FLevelComponent[1].UnspentSkillPoints') unspent,
			json_extract(f.components,'$.FHealthComponent[1].m_CurrentHealth') health
		from actor_fgl_entities a join fgl_entities f on f.entity_id=a.entity_id where a.actor_id=? and a.slot_name='DuneCharacter'`, p.Pawn)
	if xp, ok := toInt(lv["xp"]); ok {
		out["xp"] = xp
		out["level"] = xpToLevel(xp)
	}
	if total, ok := toInt(lv["total"]); ok {
		unspent, _ := toInt(lv["unspent"])
		out["skillPoints"] = map[string]any{"unspent": unspent, "total": total}
	}

	if in, _ := o.S.One(`select json_extract(properties,'$.TechKnowledgePlayerComponent.m_TechKnowledgePoints') points from actors where id=?`, p.Pawn); len(in) > 0 && in["points"] != nil {
		out["intel"] = map[string]any{"points": in["points"], "max": maxIntel}
	}

	gas, _ := o.S.One(`select json_extract(gas_attributes,'$.DuneHydrationAttributeSet.CurrentHydration.CurrentValue') hydration,
		json_extract(gas_attributes,'$.DuneSpiceAddictionAttributeSet.SpiceAddictionLevel.CurrentValue') addiction from actors where id=?`, p.Pawn)
	out["vitals"] = map[string]any{
		"health": lv["health"], "healthMinMax": baseMaxHealth,
		"hydration": gas["hydration"], "maxHydration": 100,
		"spiceAddiction": gas["addiction"], "maxSpiceAddiction": 10,
	}

	// Two different balances: the virtual currency (Solari Credit is id 0, the secondary wallet id 1) and the Solari Coin item
	// stacks in the inventory. The console shows both.
	tiles := []map[string]any{}
	bal := map[int64]any{}
	rows, _ := o.S.Query(`select currency_id, balance from player_virtual_currency_balances where player_controller_id=? order by currency_id`, p.Controller)
	for _, r := range rows {
		if id, ok := toInt(r["currency_id"]); ok {
			bal[id] = r["balance"]
		}
	}
	labels := map[int64]string{0: "Solari Credit", 1: "House Credit"}
	for id := int64(0); id <= 1; id++ { // the two expected wallets are always shown, 0 when absent
		b, ok := bal[id]
		if !ok {
			b = int64(0)
		}
		tiles = append(tiles, map[string]any{"label": labels[id], "balance": b})
		delete(bal, id)
	}
	for id, b := range bal {
		tiles = append(tiles, map[string]any{"label": "Currency " + strconv.FormatInt(id, 10), "balance": b})
	}
	tiles = append(tiles[:1], append([]map[string]any{{"label": "Solari Coin", "balance": o.solari(p)}}, tiles[1:]...)...)
	out["currency"] = tiles
	return out, nil
}

func toInt(v any) (int64, bool) {
	switch n := v.(type) {
	case int64:
		return n, true
	case float64:
		return int64(n), true
	}
	return 0, false
}
