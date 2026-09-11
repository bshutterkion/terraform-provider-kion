#!/usr/bin/env python3
"""Dump a Kion install's audit trail to a JSON file.

Pages through POST /api/v1/audit-trail/search, following the cursor the API
returns, and writes every record it saw to one file.

    ./arr/audit-trail-dump.py --last-hours 48 --out audit-trail.json

Two things about the endpoint's date filter, both learned by probing it: the
window must be given as *both* start_date and end_date (with one alone it
accepts the field and silently returns the whole trail), each formatted RFC3339
with a literal Z; and end_date is rounded up to the end of that day, so records
past the instant asked for come back. This script refuses a half window and
trims the overshoot (--no-trim keeps it).

Credentials come from arr/.audit-trail.env, which this script loads on its
own (and which is plain shell, so `source`ing it works the same). Environment
variables already set win over the file, so an export or `--env-file` overrides
it without editing anything.

Stdlib only — no pip install.
"""

from __future__ import annotations

import argparse
import datetime as dt
import json
import os
import shlex
import sys
import time
import urllib.error
import urllib.parse
import urllib.request

SEARCH_PATH = "/api/v1/audit-trail/search"
RETRY_STATUSES = frozenset({429, 500, 502, 503, 504})
DEFAULT_ENV_FILE = os.path.join(os.path.dirname(os.path.abspath(__file__)), ".audit-trail.env")


def load_env_file(path: str, required: bool) -> None:
    """Read KEY=value lines (with or without `export`) into os.environ.

    Already-set variables are left alone, so the shell still wins.
    """
    if not os.path.exists(path):
        if required:
            raise SystemExit(f"error: env file not found: {path}")
        return
    with open(path, encoding="utf-8") as fh:
        for lineno, raw in enumerate(fh, 1):
            line = raw.strip()
            if not line or line.startswith("#"):
                continue
            if line.startswith("export "):
                line = line[len("export ") :].lstrip()
            key, sep, value = line.partition("=")
            if not sep:
                print(f"warning: {path}:{lineno}: ignoring unparsable line", file=sys.stderr)
                continue
            key = key.strip()
            try:
                parts = shlex.split(value.strip())
            except ValueError:
                print(f"warning: {path}:{lineno}: ignoring unquotable value", file=sys.stderr)
                continue
            os.environ.setdefault(key, parts[0] if parts else "")


def base_url(raw: str) -> str:
    """Normalize --url to an origin, tolerating a trailing /api or slashes."""
    url = raw.strip().rstrip("/")
    if not url:
        raise ValueError("empty --url")
    if "://" not in url:
        url = "https://" + url
    if url.endswith("/api"):
        url = url[: -len("/api")]
    return url


def search_url(base: str, count: int, sort_method: str, sort_order: str, cursor: str | None) -> str:
    params = {
        "count": str(count),
        "sortMethod": sort_method,
        "sortOrder": sort_order,
    }
    if cursor:
        # The API only accepts a cursor alongside a direction.
        params["cursor_direction"] = "forward"
        params["cursor"] = cursor
    return f"{base}{SEARCH_PATH}?{urllib.parse.urlencode(params)}"


def post(url: str, token: str, payload: dict, timeout: float, retries: int) -> dict:
    body = json.dumps(payload).encode()
    attempt = 0
    while True:
        attempt += 1
        req = urllib.request.Request(
            url,
            data=body,
            method="POST",
            headers={
                "Authorization": f"Bearer {token}",
                "Content-Type": "application/json",
                "Accept": "application/json",
            },
        )
        try:
            with urllib.request.urlopen(req, timeout=timeout) as resp:
                return json.loads(resp.read().decode())
        except urllib.error.HTTPError as err:
            detail = err.read().decode(errors="replace")[:2000]
            if err.code in RETRY_STATUSES and attempt <= retries:
                backoff = min(2 ** (attempt - 1), 30)
                print(
                    f"  HTTP {err.code}, retrying in {backoff}s "
                    f"(attempt {attempt}/{retries})",
                    file=sys.stderr,
                )
                time.sleep(backoff)
                continue
            raise SystemExit(f"POST {url} failed: HTTP {err.code} {err.reason}\n{detail}")
        except (urllib.error.URLError, TimeoutError, OSError) as err:
            # A read timeout surfaces as a bare TimeoutError, not a URLError.
            reason = getattr(err, "reason", err) or err
            if attempt <= retries:
                backoff = min(2 ** (attempt - 1), 30)
                print(f"  {reason}, retrying in {backoff}s", file=sys.stderr)
                time.sleep(backoff)
                continue
            raise SystemExit(f"POST {url} failed: {reason}")


def api_timestamp(value: str) -> str:
    """Normalize a timestamp to the only shape the endpoint accepts.

    It wants RFC3339 with a literal Z, e.g. 2026-08-26T06:00:00.000Z; a naive
    datetime, a bare date, or an epoch is rejected with 422.
    """
    text = value.strip()
    try:
        parsed = dt.datetime.fromisoformat(text.replace("Z", "+00:00"))
    except ValueError as err:
        raise SystemExit(f"error: cannot parse timestamp {value!r}: {err}")
    if parsed.tzinfo is None:
        parsed = parsed.replace(tzinfo=dt.timezone.utc)
    parsed = parsed.astimezone(dt.timezone.utc)
    return parsed.strftime("%Y-%m-%dT%H:%M:%S.") + f"{parsed.microsecond // 1000:03d}Z"


def parse_created_at(value: str | None) -> dt.datetime | None:
    if not value:
        return None
    try:
        parsed = dt.datetime.fromisoformat(value.replace("Z", "+00:00"))
    except ValueError:
        return None
    if parsed.tzinfo is None:
        parsed = parsed.replace(tzinfo=dt.timezone.utc)
    return parsed.astimezone(dt.timezone.utc)


def resolve_window(args) -> None:
    """Fill in start_date/end_date and enforce the endpoint's all-or-nothing rule."""
    if args.last_hours is not None:
        if args.start_date or args.end_date:
            raise SystemExit("error: --last-hours cannot be combined with --start-date/--end-date")
        now = dt.datetime.now(dt.timezone.utc)
        args.start_date = api_timestamp((now - dt.timedelta(hours=args.last_hours)).isoformat())
        args.end_date = api_timestamp(now.isoformat())
        return

    if bool(args.start_date) != bool(args.end_date):
        # The API accepts one bound and then ignores the filter entirely, which
        # silently returns the whole trail. Refuse rather than mislead.
        raise SystemExit(
            "error: --start-date and --end-date must be given together; the API "
            "ignores the date filter unless both are set"
        )
    if args.start_date:
        args.start_date = api_timestamp(args.start_date)
        args.end_date = api_timestamp(args.end_date)


def decode_log_json(item: dict) -> dict:
    """Replace the log_json string with the object it encodes, when it is one."""
    raw = item.get("log_json")
    if not isinstance(raw, str) or not raw:
        return item
    try:
        item["log_json"] = json.loads(raw)
    except json.JSONDecodeError:
        pass
    return item


def fetch(args, token: str) -> tuple[list[dict], list[dict]]:
    """Page through the endpoint. Returns (items, per-page metadata)."""
    base = base_url(args.url)
    payload = {
        "query": args.query,
        "user_ids": args.user_id or [],
        "item_type": args.item_type,
        "start_date": args.start_date,
        "end_date": args.end_date,
    }

    # The server rounds end_date up to the end of that day, so a window can come
    # back with records past the instant asked for. Trim to the exact window.
    win_start = parse_created_at(args.start_date) if args.trim else None
    win_end = parse_created_at(args.end_date) if args.trim else None

    # The endpoint field reads like "PUT /api/v3/ou/42/labels"; the API has no
    # method filter of its own, so select on that verb here.
    methods = {m.upper() for m in (args.method or [])}

    items: list[dict] = []
    pages: list[dict] = []
    seen_ids: set = set()
    seen_cursors: set[str] = set()
    cursor: str | None = args.cursor
    page = 0
    trimmed = 0
    wrong_method = 0

    while True:
        page += 1
        url = search_url(base, args.count, args.sort_method, args.sort_order, cursor)
        print(f"page {page}: cursor={cursor or '-'}", file=sys.stderr)
        body = post(url, token, payload, args.timeout, args.retries)

        data = body.get("data") or {}
        batch = data.get("items") or []
        next_cursor = data.get("next_cursor") or None

        new = 0
        fresh = 0  # unique ids this page, counted before any filtering
        page_trimmed = 0
        page_method = 0
        for item in batch:
            key = item.get("id")
            if key is not None:
                if key in seen_ids:
                    continue
                seen_ids.add(key)
            fresh += 1
            if win_start or win_end:
                stamp = parse_created_at(item.get("created_at"))
                if stamp and (
                    (win_start and stamp < win_start) or (win_end and stamp > win_end)
                ):
                    trimmed += 1
                    page_trimmed += 1
                    continue
            if methods:
                endpoint = item.get("endpoint") or ""
                if endpoint.split(" ", 1)[0].upper() not in methods:
                    wrong_method += 1
                    page_method += 1
                    continue
            items.append(decode_log_json(item) if args.decode_log_json else item)
            new += 1

        pages.append(
            {
                "page": page,
                "cursor": cursor,
                "next_cursor": next_cursor,
                "returned": len(batch),
                "kept": new,
                "outside_window": page_trimmed,
                "other_method": page_method,
            }
        )
        notes = "".join(
            part
            for part, n in (
                (f", {page_trimmed} outside window", page_trimmed),
                (f", {page_method} other method", page_method),
            )
            if n
        )
        print(f"  {len(batch)} records ({new} kept{notes}), total {len(items)}", file=sys.stderr)

        if args.max_items and len(items) >= args.max_items:
            del items[args.max_items :]
            print(f"  stopping: reached --max-items {args.max_items}", file=sys.stderr)
            break
        if args.max_pages and page >= args.max_pages:
            print(f"  stopping: reached --max-pages {args.max_pages}", file=sys.stderr)
            break
        if not batch or not next_cursor:
            print("  stopping: no further cursor", file=sys.stderr)
            break
        if fresh == 0:
            # Every record repeated — the cursor is not advancing.
            print("  stopping: page returned nothing new", file=sys.stderr)
            break
        if next_cursor in seen_cursors:
            print(f"  stopping: cursor {next_cursor} already visited", file=sys.stderr)
            break
        seen_cursors.add(next_cursor)
        cursor = next_cursor
        if args.sleep:
            time.sleep(args.sleep)

    if trimmed:
        print(f"dropped {trimmed} record(s) outside the requested window", file=sys.stderr)
    if wrong_method:
        print(
            f"dropped {wrong_method} record(s) not matching --method "
            f"{'/'.join(sorted(methods))}",
            file=sys.stderr,
        )
    return items, pages


def parse_args(argv: list[str]) -> argparse.Namespace:
    p = argparse.ArgumentParser(
        description="Dump a Kion audit trail to JSON, following the API's cursor."
    )
    p.add_argument(
        "--env-file",
        default=DEFAULT_ENV_FILE,
        help="shell-style file holding KION_API_URL / KION_AUTH_TOKEN "
        "(default: arr/.audit-trail.env)",
    )
    p.add_argument(
        "--url",
        default=None,
        help="Kion base URL, e.g. https://demo1.kion.io (env KION_API_URL). "
        "A trailing /api is accepted and ignored.",
    )
    p.add_argument("--out", default="audit-trail.json", help="output file (default: %(default)s)")
    p.add_argument("--count", type=int, default=100, help="page size (default: %(default)s)")
    p.add_argument(
        "--user-id",
        type=int,
        action="append",
        metavar="ID",
        help="filter to a user id; repeat for several. Omit for all users.",
    )
    p.add_argument("--query", default="", help="free-text search")
    p.add_argument("--item-type", default="", help="item_type filter, e.g. login, app_label")
    p.add_argument(
        "--method",
        action="append",
        metavar="VERB",
        help="keep only records whose endpoint uses this HTTP verb, e.g. PUT. "
        "Repeat for several. Applied here, not by the API.",
    )
    p.add_argument(
        "--last-hours",
        type=float,
        default=None,
        metavar="N",
        help="shorthand for a window covering the last N hours up to now",
    )
    p.add_argument(
        "--start-date",
        default=None,
        help="window start, e.g. 2026-08-24T21:00:00Z. Must be paired with --end-date: "
        "the API ignores the date filter unless both are set.",
    )
    p.add_argument("--end-date", default=None, help="window end; must be paired with --start-date")
    p.add_argument(
        "--no-trim",
        dest="trim",
        action="store_false",
        help="keep records the API returns outside the window (it rounds end_date "
        "up to the end of that day)",
    )
    p.add_argument("--sort-method", default="alphabetical", help="default: %(default)s")
    p.add_argument("--sort-order", default="ASC", choices=("ASC", "DESC"))
    p.add_argument("--cursor", default=None, help="resume from this cursor")
    p.add_argument("--max-pages", type=int, default=0, help="stop after N pages (0 = no limit)")
    p.add_argument("--max-items", type=int, default=0, help="stop after N records (0 = no limit)")
    p.add_argument(
        "--decode-log-json",
        action="store_true",
        help="parse each record's log_json string into a nested object",
    )
    p.add_argument("--sleep", type=float, default=0.0, help="seconds to wait between pages")
    p.add_argument("--timeout", type=float, default=60.0, help="per-request timeout in seconds")
    p.add_argument("--retries", type=int, default=3, help="retries on 429/5xx and network errors")
    return p.parse_args(argv)


def main(argv: list[str]) -> int:
    args = parse_args(argv)
    load_env_file(args.env_file, required=args.env_file != DEFAULT_ENV_FILE)
    resolve_window(args)

    args.url = args.url or os.environ.get("KION_API_URL", "")
    if not args.url:
        print(f"error: set KION_API_URL in {args.env_file}, or pass --url", file=sys.stderr)
        return 2

    token = (os.environ.get("KION_AUTH_TOKEN") or os.environ.get("KION_API_KEY") or "").strip()
    if token.lower().startswith("bearer "):
        token = token[len("bearer ") :].strip()
    if not token:
        print(f"error: set KION_AUTH_TOKEN in {args.env_file}", file=sys.stderr)
        return 2

    items, pages = fetch(args, token)

    out = {
        "source": {
            "url": base_url(args.url) + SEARCH_PATH,
            "count": args.count,
            "sort_method": args.sort_method,
            "sort_order": args.sort_order,
            "filters": {
                "query": args.query,
                "user_ids": args.user_id,
                "item_type": args.item_type,
                "start_date": args.start_date,
                "end_date": args.end_date,
                "method": args.method,
            },
            "trimmed_to_window": bool(args.trim and args.start_date),
            "fetched_at": api_timestamp(dt.datetime.now(dt.timezone.utc).isoformat()),
        },
        "pages": pages,
        "total": len(items),
        "items": items,
    }
    with open(args.out, "w", encoding="utf-8") as fh:
        json.dump(out, fh, indent=2, ensure_ascii=False)
        fh.write("\n")

    print(f"wrote {len(items)} records over {len(pages)} page(s) to {args.out}", file=sys.stderr)
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
