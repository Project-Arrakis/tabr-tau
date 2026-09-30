#!/usr/bin/env python3
"""snapdiff: row- and JSON-path-level diff of two Dune: Awakening single-player saves.

Inputs (either side): a game.db / autosave/N.bak (8-byte header + zlib SQLite),
a plain decoded SQLite file, or a snapshot folder containing game.db.

Read-only: never writes to the inputs. Needs Python 3.8+ and SQLite >= 3.45 (for jsonb).
Usage: snapdiff.py BEFORE AFTER [--tables t1,t2] [--max-rows 40] [--ignore-tables a,b]
"""
import argparse, json, os, sqlite3, struct, sys, tempfile, zlib


def load(path):
    if os.path.isdir(path):
        path = os.path.join(path, "game.db")
    b = open(path, "rb").read()
    if b[:16] == b"SQLite format 3\x00":
        raw = b
    else:
        tag, size = struct.unpack("<II", b[:8])
        if tag != 1:
            sys.exit(f"{path}: unexpected header tag {tag}")
        raw = zlib.decompress(b[8:])
        if len(raw) != size:
            sys.exit(f"{path}: size mismatch {len(raw)} != {size}")
    c = sqlite3.connect(":memory:")
    c.deserialize(raw)
    c.execute("pragma query_only=on")
    return c


def tables(c):
    return [r[0] for r in c.execute("select name from sqlite_master where type='table' and name not like 'sqlite_%'")]


def cols(c, t):
    return [(r[1], r[5]) for r in c.execute(f'pragma table_info("{t}")')]


def is_json_blob(c, t, col):
    try:
        r = c.execute(f'select json("{col}") from "{t}" where typeof("{col}")=\'blob\' limit 1').fetchone()
        return r is not None
    except sqlite3.Error:
        return False


def rows(c, t):
    cs = cols(c, t)
    names = [n for n, _ in cs]
    pk = [n for n, p in sorted(cs, key=lambda x: x[1]) if p]
    jb = [n for n in names if is_json_blob(c, t, n)]
    sel = ", ".join((f'json("{n}")' if n in jb else f'"{n}"') for n in names)
    out = {}
    for i, r in enumerate(c.execute(f'select {sel} from "{t}"')):
        d = dict(zip(names, r))
        for n in jb:
            if isinstance(d[n], str):
                try:
                    d[n] = json.loads(d[n])
                except ValueError:
                    pass
        key = tuple(d[k] for k in pk) if pk else (i,)
        out[key] = d
    return out, pk


def jdiff(a, b, path=""):
    if isinstance(a, dict) and isinstance(b, dict):
        for k in sorted(set(a) | set(b)):
            p = f"{path}.{k}" if path else str(k)
            if k not in a:
                yield ("+", p, None, b[k])
            elif k not in b:
                yield ("-", p, a[k], None)
            else:
                yield from jdiff(a[k], b[k], p)
    elif isinstance(a, list) and isinstance(b, list) and len(a) == len(b):
        for i, (x, y) in enumerate(zip(a, b)):
            yield from jdiff(x, y, f"{path}[{i}]")
    elif a != b:
        yield ("~", path, a, b)


def short(v, n=120):
    s = json.dumps(v, default=str) if not isinstance(v, str) else v
    return s if len(s) <= n else s[: n - 3] + "..."


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("before"); ap.add_argument("after")
    ap.add_argument("--tables"); ap.add_argument("--ignore-tables", default="")
    ap.add_argument("--max-rows", type=int, default=40)
    a = ap.parse_args()
    A, B = load(a.before), load(a.after)
    ta, tb = set(tables(A)), set(tables(B))
    only = set(a.tables.split(",")) if a.tables else None
    ign = set(filter(None, a.ignore_tables.split(",")))
    if ta ^ tb:
        print("TABLE SET DIFFERS:", sorted(ta - tb), sorted(tb - ta))
    changed = 0
    for t in sorted(ta & tb):
        if (only and t not in only) or t in ign:
            continue
        ra, pk = rows(A, t); rb, _ = rows(B, t)
        add = [k for k in rb if k not in ra]; rem = [k for k in ra if k not in rb]
        mod = [k for k in ra if k in rb and ra[k] != rb[k]]
        if not (add or rem or mod):
            continue
        changed += 1
        print(f"\n## {t}  (pk={pk or 'rowid-order'})  +{len(add)} -{len(rem)} ~{len(mod)}")
        for k in add[: a.max_rows]:
            print(f"  + {k}: {short(rb[k])}")
        for k in rem[: a.max_rows]:
            print(f"  - {k}: {short(ra[k])}")
        for k in mod[: a.max_rows]:
            print(f"  ~ {k}")
            for kind, p, x, y in jdiff(ra[k], rb[k]):
                print(f"      {kind} {p}: {short(x)} -> {short(y)}")
    print(f"\n{changed} table(s) changed")


if __name__ == "__main__":
    main()
