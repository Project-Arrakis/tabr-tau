#!/usr/bin/env python3
"""Generate the feature inventory: every route of the self-host console against tabr-tau.

    python tools/featureinv.py <path-to>/docs/console/API-REFERENCE.md [internal/web/server.go] > docs/design/feature-inventory.md

The console side is parsed from its API reference (routes only: method, path, section; descriptions are not copied).
The tabr-tau side is parsed from internal/web/server.go. The status of each console route comes from the RULES table
below, which is the reviewed decision; a route no rule matches is listed as UNREVIEWED so gaps in the review show up.
Statuses: Done, Partial, Gap, Cut (dropped on purpose), N/A (server-only or multiplayer-only).
"""
import re
import sys
from collections import Counter, OrderedDict

# (regex on "METHOD /path", status, tabr-tau feature or reason). First match wins; paths use {} for any {param}.
RULES = [
    # --- Players: roster and server-side (single character, no server)
    (r"^GET /api/players(/list-settings|/online|/search|/deleted-characters)?$|^POST /api/players/list-settings$", "N/A", "player roster; one character in single-player"),
    (r"/(kick|ban|repair-login-queue)$|/kick-all-online$|/ban$", "N/A", "server moderation"),
    (r"/(events|stats|history)$", "N/A", "unsupported in the console itself"),
    # --- Players: profile and inventory
    (r"^GET /api/players/\{\}$|/currency$|/solaris-coin$", "Done", "Player tab (profile, Solari)"),
    (r"^GET /api/players/\{\}/inventory$", "Done", "Player > inventory"),
    (r"^POST /api/players/\{\}/(give-item|give-items|give-item-id)$", "Done", "Give item (catalog picker); quality/grade/augments not set (see augment-item)"),
    (r"^PATCH /api/players/\{\}/inventory/\{\}$|^DELETE /api/players/\{\}/inventory/\{\}$", "Done", "edit / delete item"),
    (r"^POST /api/players/\{\}/repair-gear$", "Done", "Repair gear"),
    (r"^POST /api/players/\{\}/refill-water$", "Partial", "Refill containers (held water containers); the hydration stat itself is not edited"),
    (r"/clean-inventory$", "Gap", "remove invalid items; maintenance"),
    (r"/augment-item$", "Done", "augments editor on Character > Inventory (and in Give item)"),
    (r"/add-currency$", "Done", "Give currency (Solari Credit, House Credit wallets); the Solari Coin stack is a separate item"),
    # --- Players: progression
    (r"^GET /api/players/\{\}/progression$|^POST /api/players/\{\}/(add-xp|set-skill-points|set-skill-module)$", "Done", "Give XP, Set unspent points and the rank control on Player > Skills; the Player summary shows level, XP and points"),
    (r"^POST /api/players/\{\}/reset-progression$", "Partial", "no single route: one reset button per area, each with a typed confirmation (#339). Done: Player > Skills (all skills to rank 0), Character > Overview (level and XP), Player > Specialization (XP of the known tracks). Not yet: Journey. Research has none (the save does not say what a new character has researched)"),
    (r"/vitals$", "Partial", "health, hydration and spice addiction are shown in the Player summary, read-only; the save keeps no maximum"),
    (r"/factions$|/add-faction-reputation$", "Done", "faction reputation (P10)"),
    (r"POST /api/players/\{\}/faction$", "Done", "Faction Assignment on Player > Admin (Neutral, Atreides, Harkonnen)"),
    (r"/intel$|/add-intel$", "Done", "Give Intel (Quick rewards; capped at 2,779) and the Intel figure in the Player summary"),
    (r"/specs$", "Done", "specialization tracks (P11)"),
    (r"/specializations/", "Done", "Player > Specialization: Add XP, Grant Max, Reset per track, Grant / Reset All Keystones (needs each track number assigned once)"),
    (r"/crafting-recipes", "Done", "Player > Crafting lists known recipes and the ones research can still give; Unlock"),
    (r"/research-items", "Done", "Player > Research lists the research tree; Unlock, Repair unlock"),
    (r"/journey", "Done", "Journey nodes (P12)"),
    (r"/tutorials/", "Done", "tutorials (P12)"),
    (r"^(GET|POST) /api/players/\{\}/(position|teleport)$", "Done", "Teleport (P9); destinations picker in #67"),
    (r"^POST /api/map/teleport-player$", "Done", "Teleport (P9)"),
    (r"/spawn-vehicle$", "Gap", "spawning a vehicle needs the game to create it correctly (hard)"),
    (r"/repair-vehicle-decay$", "Done", "Repair Vehicle Durability: every module made as good as new, Repair Below % threshold"),
    (r"^POST /api/players/\{\}/refuel-vehicle$", "Gap", "refuel; fuel field not located in a save yet"),
    (r"^GET /api/players/\{\}/vehicles$", "Done", "Player > Vehicles lists your own vehicles (#92)"),
    (r"^GET /api/players/\{\}/bases$", "Done", "Player > Bases lists your bases (single player has no roster)"),
    (r"^/api/guilds|GET /api/guilds", "N/A", "guilds; the save only holds simulated Landsraad guilds"),
    # --- Bases
    (r"^GET /api/bases$", "Done", "Bases tab"),
    (r"^GET /api/bases/\{\}/(export|export-backup)$|/api/base-backups", "Gap", "base export / base backups (picked-up bases): candidate feature, formats unmapped"),
    (r"/refill-generators$", "Done", "Refill generators (B4)"),
    (r"/refill-water$", "Done", "Refill base water (B4)"),
    (r"auto-refill|pending-refills|pending-water-refills|queued-refill|queued-water-refill", "Partial", "one 'Automatic refill when the editor opens' switch; queued / per-base refills are server concepts"),
    (r"^GET /api/bases/\{\}/water$", "Done", "Bases > Water: water devices with stored, capacity and fill; Refill base water"),
    (r"^GET /api/bases/\{\}/inventory$", "Done", "Bases > Inventory: the base containers in the console groups (#75, #96)"),
    (r"^GET /api/bases/\{\}/containers/\{\}$", "Done", "Storage > Open"),
    (r"/containers/\{\}/items/\{\}$", "Done", "remove item from a container"),
    (r"/containers/\{\}/(items|give-item|give-items|fill-item)$", "Done", "add item to a container (picker parity: #73)"),
    (r"permissions|permission-candidates|system-custodian|child-access|queued-child-access", "Cut", "multiplayer permissions; single owner (#58)"),
    (r"^DELETE /api/bases/\{\}$|/api/bases/pending-deletes|/api/bases/\{\}/queued-delete", "Cut", "delete a base (#58)"),
    (r"^GET /api/storage$|^GET /api/storage/\{\}$|^GET /api/storage/\{\}/items$", "Done", "Bases > Storage"),
    (r"^POST /api/storage/\{\}/give-item$", "Done", "add item to storage"),
    (r"^GET /api/storage/\{\}/export$", "Gap", "export one container (low)"),
    # --- Vehicles
    (r"^GET /api/vehicles$", "Done", "Player > Vehicles: type, lowest condition, fuel (current amount; the save has no tank size) and location"),
    (r"permissions|permission-candidates|system-custodian", "Cut", "vehicle permissions (#58)"),
    (r"^GET /api/vehicles/\{\}/storage$", "Partial", "cargo hold shown on the vehicle details, read-only"),
    (r"^DELETE /api/vehicles/\{\}/storage/", "Gap", "delete cargo stacks"),
    (r"^DELETE /api/vehicles/\{\}(/stored|/queued-delete)?$|/api/vehicles/pending-deletes", "Cut", "delete a vehicle (#58)"),
    # --- Blueprints
    (r"^[A-Z]+ /api/blueprints", "Gap", "blueprints import / export; format unmapped"),
    # --- Market / exchange
    (r"^[A-Z]+ /api/exchange", "Cut", "CHOAM exchange and market bot (#58); vendor limits kept under Player > Vendors"),
    # --- Landsraad and admin
    (r"^GET /api/admin/landsraad$", "Done", "Landsraad tab"),
    (r"^POST /api/admin/landsraad/(task-goal|term-task-goals|reward-tier|milestone-preset|player-contribution)$|^GET /api/admin/landsraad/milestone-preset$", "Gap", "goal amounts, reward tiers, contribution; tabr-tau completes / reopens tasks and sets progress"),
    (r"^GET /api/admin/(items|items/catalog|items/search)$", "Done", "item catalog picker (coverage gaps: #72)"),
    (r"^GET /api/admin/vehicles", "Partial", "vehicle class names only"),
    (r"^GET /api/admin/skill-modules$", "Done", "Player > Skills uses the console skill catalog"),
    (r"^GET /api/admin/history$", "Partial", "edits waiting for the file are listed in the banner; no persistent command history"),
    (r"^POST /api/admin/history/clear$", "N/A", "command history"),
    (r"^[A-Z]+ /api/admin/(character-transfer|message-of-the-day|player-announcements|map-chat|broadcast)", "N/A", "server messaging"),
    # --- Database
    (r"^[A-Z]+ /api/database/(tables|table|search|query|export|status)", "Done", "Database tab (browse, edit, SQL, export)"),
    (r"^[A-Z]+ /api/database/(schemas|routines|password)", "N/A", "Postgres schemas, routines, password"),
    (r"^PATCH /api/database/", "Done", "Database tab row edit"),
    # --- World / maps / live map
    (r"^GET /api/maps/spicefields|^PATCH /api/maps/spicefields", "Gap", "spicefield settings; location in the save to confirm"),
    (r"user-settings|userengine|usergame", "Partial", "Config tab edits the .ini files; per-map layering is server-side (#77)"),
    (r"^[A-Z]+ /api/map/(markers|spice|poi|players|bases|storage|services)$", "Partial", "Live Map tab draws your character, vehicles, base, storage, map markers, spice fields and flour sand from the save (not live; teleport from the map is open, #67)"),
    (r"^[A-Z]+ /api/map/", "N/A", "live-map plumbing"),
    (r"^[A-Z]+ /api/(maps|sietches|deepdesert)", "N/A", "map servers, memory, autoscaler, Sietches, Deep Desert"),
    # --- Care package
    (r"^[A-Z]+ /api/care-package", "Gap", "automatic starter kits; could be a one-click kit (low)"),
    # --- Server and platform (all N/A)
    (r"^[A-Z]+ /api/backups", "N/A", "server database backups; tabr-tau has its own save backups and restore"),
    (r"^[A-Z]+ /api/(server|updates|console|logs|settings|setup|auth|iam|api-keys|addons|discord|integrations|metrics|bot)", "N/A", "server operation, accounts, keys, addons, Discord"),
]

SECTION_DEFAULT = {  # whole sections that never apply to a single-player editor
    "Server Operations", "Updates", "Backups", "Authentication & Setup", "Settings & Public Directory", "Logs & Monitoring",
    "API Keys", "IAM Policies", "Addons", "Discord Adapter (Experimental)",
}


def norm(path):
    return re.sub(r"\{[^}]+\}", "{}", path)


def console_routes(ref):
    rows, sec = [], ""
    for ln in open(ref, encoding="utf-8"):
        m = re.match(r"^## (.*)", ln)
        if m:
            sec = m.group(1).strip()
        m = re.match(r"^\|\s*`?(GET|POST|PUT|PATCH|DELETE)`?\s*\|\s*`([^`]+)`", ln)
        if m:
            rows.append((sec, m.group(1), norm(m.group(2))))
    return rows


def classify(sec, method, path):
    key = f"{method} {path}"
    for rx, status, note in RULES:
        if re.search(rx, key):
            return status, note
    if sec in SECTION_DEFAULT:
        return "N/A", "server-only section"
    return "UNREVIEWED", ""


def tabr_routes(server_go):
    src = open(server_go, encoding="utf-8").read()
    out = sorted(set(re.findall(r'"((?:GET |POST |PUT |PATCH |DELETE )?/api/[^"]+)"', src)))
    return out


def main():
    if len(sys.argv) not in (2, 3):
        sys.exit(__doc__)
    server_go = sys.argv[2] if len(sys.argv) == 3 else "internal/web/server.go"
    rows = console_routes(sys.argv[1])
    by_sec = OrderedDict()
    for sec, method, path in rows:
        st, note = classify(sec, method, path)
        by_sec.setdefault(sec, []).append((method, path, st, note))
    total = Counter(st for v in by_sec.values() for _, _, st, _ in v)
    p = print
    p("# Feature inventory: self-host console routes vs tabr-tau\n")
    p("Generated by `tools/featureinv.py` from the console's API reference (routes only; no descriptions copied) and")
    p("`internal/web/server.go`. The status of each route is the reviewed rule set in the script; `UNREVIEWED` means no rule")
    p("matched, so the review is incomplete there. Summary and reading guide: `2026-10-08-feature-comparison-selfhost-console.md`.\n")
    p(f"Console routes parsed: **{len(rows)}**. " + ", ".join(f"{k} {v}" for k, v in total.most_common()) + ".\n")
    p("| Section | Routes | Done | Partial | Gap | Cut | N/A | Unreviewed |")
    p("|---|---:|---:|---:|---:|---:|---:|---:|")
    for sec, v in by_sec.items():
        c = Counter(st for _, _, st, _ in v)
        p(f"| {sec} | {len(v)} | {c['Done']} | {c['Partial']} | {c['Gap']} | {c['Cut']} | {c['N/A']} | {c['UNREVIEWED']} |")
    p("")
    for sec, v in by_sec.items():
        p(f"## {sec}\n")
        p("| Method | Console route | Status | tabr-tau feature / reason |")
        p("|---|---|---|---|")
        for method, path, st, note in v:
            p(f"| {method} | `{path}` | {st} | {note} |")
        p("")
    p("## tabr-tau routes (for the reverse check)\n")
    p("Routes tabr-tau serves today (`internal/web/server.go`). Anything here with no console counterpart is tabr-tau-only: the")
    p("review / commit / restore pipeline, `.ini` editing, land-claim expand and shrink, base repair and sand clearing, vendor")
    p("resets, write SQL and the automatic refill.\n")
    for r in tabr_routes(server_go):
        p(f"- `{r}`")


if __name__ == "__main__":
    main()
