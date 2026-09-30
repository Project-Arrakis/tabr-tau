#!/usr/bin/env python3
"""snapdiff: row- and JSON-path-level diff of two Dune: Awakening single-player saves.

Inputs (either side): game.db / autosave/N.bak (8-byte header + zlib SQLite), a plain decoded SQLite
file, or a snapshot folder containing game.db.

Read-only: never writes to the inputs. Requires Python >= 3.11 (sqlite3.Connection.deserialize) and
SQLite >= 3.45 (jsonb). Identifiers (platform/funcom IDs, 15+ digit numbers, account names) are
redacted by default; pass --no-redact only for private local analysis.

Usage: snapdiff.py BEFORE AFTER [--tables a,b] [--ignore-tables a,b]
                   [--ignore-columns table.col,col] [--noise] [--float-eps 1e-6]
                   [--max-rows 40] [--no-redact]
"""
import argparse, hashlib, json, os, re, sqlite3, struct, sys, zlib

if sys.version_info < (3, 11):
    sys.exit("snapdiff needs Python >= 3.11 (sqlite3.Connection.deserialize)")

MAX_DECODED = 256 * 1024 * 1024
# Columns that change without any player action (measured from the noise-control runs; refine in docs/evidence).
NOISE_COLUMNS = {"actors.serial", "farm_variables.universe_time_timestamp",
                 "farm_variables.universe_lastactive_timestamp", "farm_variables.down_time_accumulation",
                 "player_state.last_avatar_activity", "player_state.last_login_time"}
REDACT_COLUMNS = {"platform_id", "funcom_id", "platform_name", "user", "character_name"}
LONG_NUM = re.compile(r"\d{15,}")


def load(path):
    if os.path.isdir(path):
        path = os.path.join(path, "game.db")
    b = open(path, "rb").read()
    if b[:16] == b"SQLite format 3\x00":
        raw = b
    else:
        if len(b) < 10:
            sys.exit(f"{path}: too short to be a save")
        tag, size = struct.unpack("<II", b[:8])
        if tag != 1:
            sys.exit(f"{path}: unexpected header tag {tag} (not a Dune save wrapper)")
        if size > MAX_DECODED:
            sys.exit(f"{path}: declared size {size} exceeds cap {MAX_DECODED}")
        d = zlib.decompressobj()
        raw = d.decompress(b[8:], size + 1)
        if len(raw) != size or d.unconsumed_tail:
            sys.exit(f"{path}: decoded size {len(raw)} != header size {size} (or oversized stream)")
    if raw[:16] != b"SQLite format 3\x00":
        sys.exit(f"{path}: payload is not SQLite")
    c = sqlite3.connect(":memory:")
    c.deserialize(raw)
    c.execute("pragma query_only=on")
    return c


def tables(c):
    return [r[0] for r in c.execute("select name from sqlite_master where type='table' and name not like 'sqlite_%'")]


def qi(name):
    return '"' + name.replace('"', '""') + '"'


def blob_columns(c, t, names):
    """Return {col: 'jsonb'|'opaque'} for every column that holds BLOBs (checked on all rows)."""
    out = {}
    for n in names:
        try:
            r = c.execute(f"select count(*), sum(json_valid({qi(n)},8)) from {qi(t)} where typeof({qi(n)})='blob'").fetchone()
        except sqlite3.Error:
            continue
        if r[0]:
            out[n] = "jsonb" if r[0] == (r[1] or 0) else "opaque"
    return out


def redact(name, v, on):
    if not on:
        return v
    if name in REDACT_COLUMNS and v not in (None, ""):
        return "<redacted>"
    if isinstance(v, str):
        return LONG_NUM.sub("<id>", v)
    return v


def rows(c, t, redact_on):
    cinfo = list(c.execute(f"pragma table_info({qi(t)})"))
    names = [r[1] for r in cinfo]
    pk = [r[1] for r in sorted(cinfo, key=lambda r: r[5]) if r[5]]
    kinds = blob_columns(c, t, names)
    sel = ", ".join((f"json({qi(n)})" if kinds.get(n) == "jsonb" else (f"hex({qi(n)})" if kinds.get(n) == "opaque" else qi(n))) for n in names)
    out, multiset = {}, {}
    for r in c.execute(f"select {sel} from {qi(t)}"):
        d = {}
        for n, v in zip(names, r):
            if kinds.get(n) == "jsonb" and isinstance(v, str):
                try:
                    v = json.loads(v)
                except ValueError:
                    v = "<malformed-jsonb>"
            d[n] = redact(n, v, redact_on)
        if pk:
            out[tuple(d[k] for k in pk)] = d
        else:  # no primary key: identity is the full row content (multiset)
            h = hashlib.sha1(json.dumps(d, sort_keys=True, default=str).encode()).hexdigest()[:12]
            multiset.setdefault(h, []).append(d)
    if not pk:
        for h, lst in multiset.items():
            for i, d in enumerate(lst):
                out[(h, i)] = d
    return out, (pk or ["<row-hash>"])


def jdiff(a, b, path, eps):
    if isinstance(a, dict) and isinstance(b, dict):
        for k in sorted(set(a) | set(b), key=str):
            p = f"{path}.{k}" if path else str(k)
            if k not in a:
                yield ("+", p, None, b[k])
            elif k not in b:
                yield ("-", p, a[k], None)
            else:
                yield from jdiff(a[k], b[k], p, eps)
    elif isinstance(a, list) and isinstance(b, list):
        if len(a) != len(b):
            yield ("~", path + f"[len {len(a)}->{len(b)}]", _short(a), _short(b))
        else:
            for i, (x, y) in enumerate(zip(a, b)):
                yield from jdiff(x, y, f"{path}[{i}]", eps)
    elif isinstance(a, float) and isinstance(b, float):
        if abs(a - b) > eps * max(1.0, abs(a), abs(b)):
            yield ("~", path, a, b)
    elif a != b:
        yield ("~", path, a, b)


def _short(v, n=120):
    s = v if isinstance(v, str) else json.dumps(v, default=str)
    return s if len(s) <= n else s[: n - 3] + "..."


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("before"); ap.add_argument("after")
    ap.add_argument("--tables"); ap.add_argument("--ignore-tables", default="")
    ap.add_argument("--ignore-columns", default=""); ap.add_argument("--noise", action="store_true")
    ap.add_argument("--float-eps", type=float, default=1e-6)
    ap.add_argument("--max-rows", type=int, default=40); ap.add_argument("--no-redact", action="store_true")
    a = ap.parse_args()
    A, B = load(a.before), load(a.after)
    ta, tb = set(tables(A)), set(tables(B))
    only = set(a.tables.split(",")) if a.tables else None
    ign_t = set(filter(None, a.ignore_tables.split(",")))
    ign_c = set(filter(None, a.ignore_columns.split(",")))
    if a.noise:
        ign_c |= NOISE_COLUMNS
    red = not a.no_redact
    if ta ^ tb:
        print("TABLE SET DIFFERS: only-before", sorted(ta - tb), "only-after", sorted(tb - ta))

    def dropcols(t, d):
        return {k: v for k, v in d.items() if k not in ign_c and f"{t}.{k}" not in ign_c}

    changed = 0
    for t in sorted(ta & tb):
        if (only and t not in only) or t in ign_t:
            continue
        ra, pk = rows(A, t, red); rb, _ = rows(B, t, red)
        add = [k for k in rb if k not in ra]; rem = [k for k in ra if k not in rb]
        mod = [k for k in ra if k in rb and dropcols(t, ra[k]) != dropcols(t, rb[k])
               and any(True for _ in jdiff(dropcols(t, ra[k]), dropcols(t, rb[k]), "", a.float_eps))]
        if not (add or rem or mod):
            continue
        changed += 1
        print(f"\n## {t}  (key={pk})  +{len(add)} -{len(rem)} ~{len(mod)}")
        for k in add[: a.max_rows]:
            print(f"  + {k}: {_short(rb[k])}")
        for k in rem[: a.max_rows]:
            print(f"  - {k}: {_short(ra[k])}")
        for k in mod[: a.max_rows]:
            print(f"  ~ {k}")
            for kind, p, x, y in jdiff(dropcols(t, ra[k]), dropcols(t, rb[k]), "", a.float_eps):
                print(f"      {kind} {p}: {_short(x)} -> {_short(y)}")
        for lst, tag in ((add, "+"), (rem, "-"), (mod, "~")):
            if len(lst) > a.max_rows:
                print(f"  ... {len(lst) - a.max_rows} more {tag} rows not shown")
    print(f"\n{changed} table(s) changed" + ("" if red else "  (UNREDACTED)"))


if __name__ == "__main__":
    main()
