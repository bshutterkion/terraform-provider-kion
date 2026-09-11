#!/usr/bin/env python3
"""Put labels back on resources whose label set was cleared, using the audit trail.

Reads a dump produced by arr/audit-trail-dump.py, reconstructs each
resource's label set as of just before it was cleared, and PUTs it back.

    # what it would do, touching nothing (default):
    ./arr/audit-trail-restore-labels.py --dump audit-trail-label-puts-all.json

    # actually write:
    ./arr/audit-trail-restore-labels.py --dump audit-trail-label-puts-all.json --apply

How the target state is derived: label PUTs are full replacements, and the audit
record stores the *new* set, not the old one. So a resource's pre-clear state is
the newest non-empty PUT that precedes its clearing PUT. A resource with no such
record cannot be restored from the audit trail and is reported, never guessed at.

Safety: dry-run unless --apply. Before writing, each resource's live labels are
read back; anything that is no longer empty is skipped, so a label set someone
restored by hand (or that was never cleared) is not clobbered. --force overrides
that check. Every run writes a plan/result file.

Credentials come from arr/.audit-trail.env, as with audit-trail-dump.py.
Stdlib only.
"""

from __future__ import annotations

import argparse
import datetime as dt
import json
import os
import re
import sys
import time
import urllib.error
import urllib.parse
import urllib.request

DEFAULT_ENV_FILE = os.path.join(os.path.dirname(os.path.abspath(__file__)), ".audit-trail.env")
RETRY_STATUSES = frozenset({429, 500, 502, 503, 504})

# Resource kinds this script will write, and the v3 collection each uses. The v1
# label endpoints take a different body shape and are deliberately not handled;
# a cleared resource of another kind is reported instead of half-restored.
SUPPORTED = {
    "project": "/api/v3/project/{id}/labels",
    "ou": "/api/v3/ou/{id}/labels",
    "cloud-rule": "/api/v3/cloud-rule/{id}/labels",
}

ENDPOINT_RE = re.compile(r"^(?P<method>[A-Z]+)\s+(?P<version>/api/v\d)/(?P<kind>[a-z-]+)/(?P<id>\d+)")


def load_env_file(path: str, required: bool) -> None:
    """Read KEY=value lines (with or without `export`) into os.environ."""
    import shlex

    if not os.path.exists(path):
        if required:
            raise SystemExit(f"error: env file not found: {path}")
        return
    with open(path, encoding="utf-8") as fh:
        for raw in fh:
            line = raw.strip()
            if not line or line.startswith("#"):
                continue
            if line.startswith("export "):
                line = line[len("export ") :].lstrip()
            key, sep, value = line.partition("=")
            if not sep:
                continue
            try:
                parts = shlex.split(value.strip())
            except ValueError:
                continue
            os.environ.setdefault(key.strip(), parts[0] if parts else "")


def request(method: str, url: str, token: str, payload, timeout: float, retries: int):
    body = json.dumps(payload).encode() if payload is not None else None
    attempt = 0
    while True:
        attempt += 1
        req = urllib.request.Request(
            url,
            data=body,
            method=method,
            headers={
                "Authorization": f"Bearer {token}",
                "Content-Type": "application/json",
                "Accept": "application/json",
            },
        )
        try:
            with urllib.request.urlopen(req, timeout=timeout) as resp:
                text = resp.read().decode()
                return resp.status, (json.loads(text) if text.strip() else {})
        except urllib.error.HTTPError as err:
            detail = err.read().decode(errors="replace")[:500]
            if err.code in RETRY_STATUSES and attempt <= retries:
                time.sleep(min(2 ** (attempt - 1), 30))
                continue
            return err.code, {"error": detail}
        except (urllib.error.URLError, TimeoutError, OSError) as err:
            reason = getattr(err, "reason", err) or err
            if attempt <= retries:
                time.sleep(min(2 ** (attempt - 1), 30))
                continue
            return 0, {"error": str(reason)}


def target_of(item: dict) -> tuple[str, int] | None:
    """(kind, id) the PUT acted on, parsed from the endpoint string."""
    match = ENDPOINT_RE.match(item.get("endpoint") or "")
    if not match:
        return None
    return match.group("kind"), int(match.group("id"))


def decoded(item: dict):
    raw = item.get("log_json")
    if isinstance(raw, str):
        try:
            return json.loads(raw)
        except json.JSONDecodeError:
            return None
    return raw


def put_labels(item: dict) -> list[dict] | None:
    """The full label set a PUT record installed, as [{key, value}].

    v3 stores {"labels": [{"id":0,"key":k,"value":v}]}; v1 stores a list of join
    rows nesting the label under "app_label". None when the record carries no
    label set, which keeps "set to empty" distinct from "not a set operation".
    """
    raw = decoded(item)
    if isinstance(raw, dict) and "labels" in raw:
        entries = raw.get("labels") or []
    elif isinstance(raw, list):
        entries = raw
    else:
        return None

    out: list[dict] = []
    for entry in entries:
        if not isinstance(entry, dict):
            continue
        label = entry.get("app_label") if isinstance(entry.get("app_label"), dict) else entry
        key, value = label.get("key"), label.get("value")
        if key is None:
            continue
        out.append({"id": 0, "key": key, "value": "" if value is None else value})
    return out


def post_label(item: dict) -> dict | None:
    """The single label a POST record added, as {key, value}."""
    raw = decoded(item)
    if not isinstance(raw, dict) or raw.get("key") is None:
        return None
    value = raw.get("value")
    return {"id": 0, "key": raw["key"], "value": "" if value is None else value}


def replay(events: list[dict]) -> tuple[list[dict] | None, bool, list[str]]:
    """Fold a resource's label events into the set in force after the last one.

    PUT replaces the whole set, POST adds one label. Returns (state, partial,
    notes): state is None when nothing in the history reveals it, and partial is
    True when the set was rebuilt from additions alone with no PUT to anchor it,
    so the result is a lower bound rather than the exact prior set.
    """
    state: list[dict] | None = None
    partial = False
    notes: list[str] = []

    for event in events:
        endpoint = event.get("endpoint") or ""
        method = endpoint.split(" ", 1)[0].upper()

        if method == "PUT":
            labels = put_labels(event)
            if labels is not None:
                # A PUT is ground truth: it replaces the set outright, which
                # also settles any ambiguity logged before it.
                state, partial, notes = labels, False, []
        elif method == "POST":
            label = post_label(event)
            if label is None:
                continue
            if state is None:
                state, partial = [], True
            if not any(x["key"] == label["key"] and x["value"] == label["value"] for x in state):
                state.append(label)
        elif method == "DELETE":
            # Association deletes name an app_label_id, which this history does
            # not map to a key/value. Flag rather than silently over-restore.
            notes.append(f"unmodelled DELETE at {event.get('created_at')}: {endpoint}")

    return state, partial, notes


def build_plan(items: list[dict], before: str | None) -> tuple[list[dict], list[dict]]:
    """Return (restorable, unrestorable) entries for every cleared resource."""
    history: dict[tuple[str, int], list[dict]] = {}
    for item in sorted(items, key=lambda i: (i.get("created_at") or "", i.get("id") or 0)):
        key = target_of(item)
        if key is None:
            continue
        history.setdefault(key, []).append(item)

    restorable: list[dict] = []
    unrestorable: list[dict] = []

    for (kind, rid), events in history.items():
        if before:
            events = [e for e in events if (e.get("created_at") or "") <= before]
            if not events:
                continue

        clear = events[-1]
        cleared_to = put_labels(clear)
        if cleared_to is None or cleared_to:
            continue  # the last thing that happened was not a clear — leave it alone

        state, partial, notes = replay(events[:-1])

        common = {
            "kind": kind,
            "id": rid,
            "cleared_at": clear.get("created_at"),
            "cleared_by": clear.get("username"),
        }
        if not state:
            unrestorable.append(
                {**common, "reason": "no label-setting record precedes the clear"}
            )
        elif kind not in SUPPORTED:
            unrestorable.append({**common, "reason": f"unsupported resource kind {kind!r}"})
        else:
            last_set = [e for e in events[:-1] if (e.get("endpoint") or "").split(" ")[0] in
                        ("PUT", "POST")]
            source = last_set[-1] if last_set else events[0]
            restorable.append(
                {
                    **common,
                    "labels": state,
                    "partial": partial,
                    "notes": notes,
                    "source_audit_id": source.get("id"),
                    "source_at": source.get("created_at"),
                    "source_by": source.get("username"),
                    "path": SUPPORTED[kind].format(id=rid),
                }
            )

    restorable.sort(key=lambda e: (e["kind"], e["id"]))
    unrestorable.sort(key=lambda e: (e["kind"], e["id"]))
    return restorable, unrestorable


def live_labels(base: str, path: str, token: str, args) -> tuple[int, list | None]:
    status, body = request("GET", base + path, token, None, args.timeout, args.retries)
    if status != 200:
        return status, None
    data = body.get("data") if isinstance(body, dict) else body
    return status, data if isinstance(data, list) else []


def main(argv: list[str]) -> int:
    p = argparse.ArgumentParser(
        description="Restore label sets cleared on Kion resources, from an audit-trail dump."
    )
    p.add_argument("--dump", required=True, help="JSON file from arr/audit-trail-dump.py")
    p.add_argument("--env-file", default=DEFAULT_ENV_FILE)
    p.add_argument("--url", default=None, help="Kion base URL (default: KION_API_URL)")
    p.add_argument(
        "--apply",
        action="store_true",
        help="actually PUT the labels back; without this nothing is written",
    )
    p.add_argument(
        "--force",
        action="store_true",
        help="restore even if the resource currently has labels (overwrites them)",
    )
    p.add_argument(
        "--before",
        default=None,
        help="only consider audit records at or before this timestamp, e.g. "
        "2026-08-25T00:00:00Z (useful to target one clearing event)",
    )
    p.add_argument("--kind", action="append", help="limit to a resource kind; repeatable")
    p.add_argument("--id", action="append", type=int, help="limit to a resource id; repeatable")
    p.add_argument("--out", default="label-restore-plan.json", help="plan/result file")
    p.add_argument("--sleep", type=float, default=0.0, help="seconds between writes")
    p.add_argument("--timeout", type=float, default=60.0)
    p.add_argument("--retries", type=int, default=3)
    args = p.parse_args(argv)

    load_env_file(args.env_file, required=args.env_file != DEFAULT_ENV_FILE)
    base = (args.url or os.environ.get("KION_API_URL", "")).strip().rstrip("/")
    if base.endswith("/api"):
        base = base[: -len("/api")]
    token = (os.environ.get("KION_AUTH_TOKEN") or os.environ.get("KION_API_KEY") or "").strip()
    if token.lower().startswith("bearer "):
        token = token[len("bearer ") :].strip()
    if not base or not token:
        print(f"error: set KION_API_URL and KION_AUTH_TOKEN in {args.env_file}", file=sys.stderr)
        return 2

    with open(args.dump, encoding="utf-8") as fh:
        dump = json.load(fh)
    items = dump.get("items") if isinstance(dump, dict) else dump
    if not isinstance(items, list):
        print(f"error: {args.dump} has no items array", file=sys.stderr)
        return 2

    # A dump taken with --method PUT has no POST records, so every label added
    # rather than set would be dropped from the reconstruction. Refuse it.
    filters = (dump.get("source") or {}).get("filters") or {} if isinstance(dump, dict) else {}
    if filters.get("method"):
        print(
            f"error: {args.dump} was filtered to method {filters['method']}; the replay needs "
            "POST records too. Re-dump without --method.",
            file=sys.stderr,
        )
        return 2
    if filters.get("item_type") not in (None, "", "app_label"):
        print(
            f"warning: dump is filtered to item_type {filters['item_type']!r}, not app_label",
            file=sys.stderr,
        )

    restorable, unrestorable = build_plan(items, args.before)

    if args.kind:
        kinds = set(args.kind)
        restorable = [e for e in restorable if e["kind"] in kinds]
        unrestorable = [e for e in unrestorable if e["kind"] in kinds]
    if args.id:
        ids = set(args.id)
        restorable = [e for e in restorable if e["id"] in ids]
        unrestorable = [e for e in unrestorable if e["id"] in ids]

    mode = "APPLY" if args.apply else "DRY RUN"
    print(f"=== {mode} — {base} ===")
    print(f"cleared resources with a recoverable prior state: {len(restorable)}")
    print(f"cleared resources with nothing to restore from  : {len(unrestorable)}")
    print()

    results = []
    counts = {"restored": 0, "would_restore": 0, "skipped": 0, "failed": 0}

    for entry in restorable:
        tag = f"{entry['kind']}/{entry['id']}"
        status, current = live_labels(base, entry["path"], token, args)
        if current is None:
            print(f"  {tag:24s} SKIP  cannot read current labels (HTTP {status})")
            results.append({**entry, "outcome": "skipped", "detail": f"GET {status}"})
            counts["skipped"] += 1
            continue
        if current and not args.force:
            print(f"  {tag:24s} SKIP  already has {len(current)} label(s); --force to overwrite")
            results.append({**entry, "outcome": "skipped", "detail": "already labelled"})
            counts["skipped"] += 1
            continue

        summary = ", ".join(f"{lbl['key']}={lbl['value']}" for lbl in entry["labels"])
        mark = " [partial]" if entry.get("partial") else ""
        if not args.apply:
            print(f"  {tag:24s} WOULD PUT {len(entry['labels'])} label(s){mark}: {summary[:80]}")
            results.append({**entry, "outcome": "would_restore"})
            counts["would_restore"] += 1
            continue

        status, body = request(
            "PUT",
            base + entry["path"],
            token,
            {"labels": entry["labels"]},
            args.timeout,
            args.retries,
        )
        if 200 <= status < 300:
            print(f"  {tag:24s} OK    restored {len(entry['labels'])} label(s)")
            results.append({**entry, "outcome": "restored", "http_status": status})
            counts["restored"] += 1
        else:
            detail = body.get("error") if isinstance(body, dict) else str(body)
            print(f"  {tag:24s} FAIL  HTTP {status} {str(detail)[:120]}")
            results.append({**entry, "outcome": "failed", "http_status": status, "detail": detail})
            counts["failed"] += 1
        if args.sleep:
            time.sleep(args.sleep)

    if unrestorable:
        print()
        print("not restorable:")
        for entry in unrestorable:
            print(f"  {entry['kind']}/{entry['id']:<8} {entry['reason']}")

    out = {
        "mode": "apply" if args.apply else "dry-run",
        "url": base,
        "dump": os.path.abspath(args.dump),
        "generated_at": dt.datetime.now(dt.timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"),
        "counts": {**counts, "unrestorable": len(unrestorable)},
        "results": results,
        "unrestorable": unrestorable,
    }
    with open(args.out, "w", encoding="utf-8") as fh:
        json.dump(out, fh, indent=2, ensure_ascii=False)
        fh.write("\n")

    print()
    print(f"{counts} -> {args.out}")
    if not args.apply and counts["would_restore"]:
        print("nothing was written; rerun with --apply to make these changes")
    return 1 if counts["failed"] else 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
