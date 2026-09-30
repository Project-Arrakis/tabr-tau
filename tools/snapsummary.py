#!/usr/bin/env python3
"""snapsummary: a REDACTED, committable fingerprint of a snapshot, for comparing copies over time.

Input: a snapshot folder (with game.db) or a game.db / .bak file.
Output: JSON with file hashes (from manifest.json when present), the decoded-database hash, per-table row
counts, applied patches, an item-template tally, and a few progression values. It contains NO account,
platform or player identifiers and no row-level data, so it is safe to commit under docs/evidence/.
Requires Python >= 3.11 and SQLite >= 3.45. Read-only.

Usage: snapsummary.py SNAPSHOT [--label NAME] [--out FILE]
"""
import argparse, hashlib, json, os, re, sqlite3, struct, sys, zlib

if sys.version_info < (3, 11):
    sys.exit("snapsummary needs Python >= 3.11")
MAX = 256 * 1024 * 1024
LONG = re.compile(r"\d{15,}")


def load(path):
    b = open(path, "rb").read()
    if b[:16] == b"SQLite format 3\x00":
        return b
    tag, size = struct.unpack("<II", b[:8])
    if tag != 1 or size > MAX:
        sys.exit(f"{path}: not a Dune save wrapper (tag {tag}, size {size})")
    d = zlib.decompressobj()
    raw = d.decompress(b[8:], size + 1)
    if len(raw) != size or d.unconsumed_tail or raw[:16] != b"SQLite format 3\x00":
        sys.exit(f"{path}: corrupt or oversized payload")
    return raw


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("snapshot"); ap.add_argument("--label"); ap.add_argument("--out")
    a = ap.parse_args()
    root = a.snapshot
    dbpath = os.path.join(root, "game.db") if os.path.isdir(root) else root
    raw = load(dbpath)
    c = sqlite3.connect(":memory:")
    c.deserialize(raw)
    c.execute("pragma query_only=on")
    q = lambda s: c.execute(s).fetchall()
    tabs = [r[0] for r in q("select name from sqlite_master where type='table' and name not like 'sqlite_%'")]
    out = {
        "label": a.label or (os.path.basename(root.rstrip("/\\")) if os.path.isdir(root) else os.path.basename(root)),
        "decodedDbSha256": hashlib.sha256(raw).hexdigest(), "decodedBytes": len(raw),
        "sqliteVersionUsed": sqlite3.sqlite_version,
        "tableRowCounts": {t: q(f'select count(*) from "{t}"')[0][0] for t in tabs},
        "appliedPatches": [r[0] for r in q("select name from applied_patches order by 1")] if "applied_patches" in tabs else [],
        "schemaHash": hashlib.sha256("\n".join(r[0] or "" for r in q("select sql from sqlite_master order by name")).encode()).hexdigest(),
    }
    items = {}
    for tid, n, cnt in q("select template_id, count(*), sum(stack_size) from items group by template_id order by 1"):
        items[tid] = {"rows": n, "totalStack": cnt}
    out["itemTemplates"] = items
    out["actorClasses"] = {k: n for k, n in q("select class, count(*) from actors group by class order by 2 desc")}
    prog = {}
    try:
        r = q("""select json(e.components) from fgl_entities e join actor_fgl_entities a on a.entity_id=e.entity_id
                 where a.slot_name='DuneCharacter' limit 1""")
        if r:
            lv = json.loads(r[0][0]).get("FLevelComponent", [0, {}])[1]
            hc = json.loads(r[0][0]).get("FHealthComponent", [0, {}])[1]
            prog.update({k: lv.get(k) for k in ("TotalXPEarned", "TotalSkillPoints", "UnspentSkillPoints", "KeystoneBonusSkillPoints")})
            prog["health"] = hc.get("m_CurrentHealth")
        r = q("select json(gas_attributes) from actors where class like '%BP_DunePlayerCharacter%' limit 1")
        if r:
            g = json.loads(r[0][0])
            prog["hydration"] = g.get("DuneHydrationAttributeSet", {}).get("CurrentHydration", {}).get("CurrentValue")
            prog["spice"] = g.get("DuneSpiceAddictionAttributeSet", {}).get("CurrentSpice", {}).get("CurrentValue")
    except (sqlite3.Error, ValueError, KeyError, IndexError):
        prog["error"] = "progression unreadable"
    out["progression"] = prog
    mf = os.path.join(root, "manifest.json") if os.path.isdir(root) else None
    if mf and os.path.exists(mf):
        m = json.load(open(mf, encoding="utf-8-sig"))
        out["manifest"] = {"takenUtc": m.get("takenUtc"), "gameRunning": m.get("gameRunning"),
                           "files": [{"path": f["path"], "bytes": f["bytes"], "sha256": f["sha256"]} for f in m.get("files", [])]}
    text = json.dumps(out, indent=1, sort_keys=True)
    if LONG.search(text):
        sys.exit("refusing to emit: output contains a 15+ digit number (possible identifier)")
    (open(a.out, "w").write(text + "\n") if a.out else print(text))


if __name__ == "__main__":
    main()
