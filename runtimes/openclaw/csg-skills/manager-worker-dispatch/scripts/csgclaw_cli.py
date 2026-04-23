#!/usr/bin/env python3
"""Small CSGClaw HTTP CLI fallback for manager-worker-dispatch.

This script covers the collaboration commands the skill needs when the Go
`csgclaw-cli` binary is not installed in the OpenClaw image.
"""

from __future__ import annotations

import argparse
import json
import os
import sys
from pathlib import Path
from typing import Any
from urllib import error, parse, request


DEFAULT_BASE_URL = "http://127.0.0.1:18080"
CONFIG_PATHS = (
    Path(os.path.expanduser("~/.openclaw/openclaw.json")),
    Path(os.path.expanduser("~/.openclaw/config.json")),
)


def load_local_settings() -> dict[str, str]:
    for path in CONFIG_PATHS:
        if not path.exists():
            continue
        try:
            data = json.loads(path.read_text(encoding="utf-8"))
        except (OSError, json.JSONDecodeError):
            continue
        channels = data.get("channels")
        if not isinstance(channels, dict):
            continue
        csgclaw = channels.get("csgclaw")
        if not isinstance(csgclaw, dict):
            continue
        settings: dict[str, str] = {}
        base_url = csgclaw.get("baseUrl") or csgclaw.get("base_url")
        access_token = csgclaw.get("accessToken") or csgclaw.get("access_token")
        if isinstance(base_url, str) and base_url.strip():
            settings["base_url"] = base_url.strip()
        if isinstance(access_token, str) and access_token.strip():
            settings["access_token"] = access_token.strip()
        return settings
    return {}


def channel_path(channel: str, resource: str) -> str:
    normalized = channel.strip().lower()
    if normalized in ("", "csgclaw"):
        return f"/api/v1/{resource}"
    if normalized == "feishu":
        return f"/api/v1/channels/feishu/{resource}"
    raise SystemExit(f'unsupported channel "{channel}"')


def room_members_path(channel: str, room_id: str) -> str:
    if not room_id.strip():
        raise SystemExit("room_id is required")
    escaped = parse.quote(room_id.strip(), safe="")
    normalized = channel.strip().lower()
    if normalized in ("", "csgclaw"):
        return f"/api/v1/rooms/{escaped}/members"
    if normalized == "feishu":
        return f"/api/v1/channels/feishu/rooms/{escaped}/members"
    raise SystemExit(f'unsupported channel "{channel}"')


class Client:
    def __init__(self, endpoint: str, token: str) -> None:
        self.endpoint = endpoint.rstrip("/") or DEFAULT_BASE_URL
        self.token = token.strip()

    def request_json(self, method: str, path: str, payload: dict[str, Any] | None = None) -> Any:
        data = None
        headers: dict[str, str] = {}
        if payload is not None:
            data = json.dumps(payload).encode("utf-8")
            headers["Content-Type"] = "application/json"
        if self.token:
            headers["Authorization"] = f"Bearer {self.token}"

        url = f"{self.endpoint}{path}"
        req = request.Request(url, data=data, method=method, headers=headers)
        try:
            with request.urlopen(req, timeout=30) as resp:
                raw = resp.read().decode("utf-8")
        except error.HTTPError as exc:
            detail = exc.read().decode("utf-8", errors="replace").strip()
            raise SystemExit(f"HTTP {exc.code} for {method} {url}: {detail}") from exc
        except error.URLError as exc:
            raise SystemExit(f"request failed for {method} {url}: {exc.reason}") from exc

        if not raw.strip():
            return {}
        try:
            return json.loads(raw)
        except json.JSONDecodeError as exc:
            raise SystemExit(f"invalid JSON response from {method} {url}: {raw}") from exc


def print_result(data: Any, output: str) -> None:
    if output == "json":
        print(json.dumps(data, ensure_ascii=False, indent=2))
        return

    if isinstance(data, list):
        for item in data:
            if isinstance(item, dict):
                print("\t".join(str(item.get(key, "")) for key in display_keys(item)))
            else:
                print(item)
        return

    if isinstance(data, dict):
        print("\t".join(str(data.get(key, "")) for key in display_keys(data)))
        return

    print(data)


def display_keys(item: dict[str, Any]) -> list[str]:
    preferred = [
        "id",
        "name",
        "description",
        "role",
        "channel",
        "agent_id",
        "user_id",
        "available",
        "handle",
        "is_online",
        "title",
    ]
    return [key for key in preferred if key in item] or sorted(item)


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(prog="csgclaw_cli.py")
    parser.add_argument("--endpoint", default="")
    parser.add_argument("--token", default="")
    parser.add_argument("--output", choices=("table", "json"), default="table")
    sub = parser.add_subparsers(dest="command", required=True)

    bot = sub.add_parser("bot")
    bot_sub = bot.add_subparsers(dest="subcommand", required=True)
    bot_list = bot_sub.add_parser("list")
    bot_list.add_argument("--channel", default="csgclaw")
    bot_list.add_argument("--role", default="")
    bot_create = bot_sub.add_parser("create")
    bot_create.add_argument("--id", default="")
    bot_create.add_argument("--name", required=True)
    bot_create.add_argument("--description", default="")
    bot_create.add_argument("--role", required=True)
    bot_create.add_argument("--channel", default="csgclaw")
    bot_create.add_argument("--model-id", default="")

    member = sub.add_parser("member")
    member_sub = member.add_subparsers(dest="subcommand", required=True)
    member_list = member_sub.add_parser("list")
    member_list.add_argument("--channel", default="csgclaw")
    member_list.add_argument("--room-id", required=True)
    member_create = member_sub.add_parser("create")
    member_create.add_argument("--channel", default="csgclaw")
    member_create.add_argument("--room-id", required=True)
    member_create.add_argument("--user-id", required=True)
    member_create.add_argument("--inviter-id", required=True)
    member_create.add_argument("--locale", default="")

    room = sub.add_parser("room")
    room_sub = room.add_subparsers(dest="subcommand", required=True)
    room_create = room_sub.add_parser("create")
    room_create.add_argument("--channel", default="csgclaw")
    room_create.add_argument("--title", required=True)
    room_create.add_argument("--description", default="")
    room_create.add_argument("--creator-id", required=True)
    room_create.add_argument("--participant-ids", default="")
    room_create.add_argument("--locale", default="")

    message = sub.add_parser("message")
    message_sub = message.add_subparsers(dest="subcommand", required=True)
    message_create = message_sub.add_parser("create")
    message_create.add_argument("--channel", default="csgclaw")
    message_create.add_argument("--room-id", required=True)
    message_create.add_argument("--sender-id", required=True)
    message_create.add_argument("--mention-id", default="")
    message_create.add_argument("--content", required=True)
    return parser


def main(argv: list[str] | None = None) -> int:
    parser = build_parser()
    args = parser.parse_args(argv)
    settings = load_local_settings()
    endpoint = args.endpoint or os.getenv("CSGCLAW_BASE_URL") or settings.get("base_url") or DEFAULT_BASE_URL
    token = args.token or os.getenv("CSGCLAW_ACCESS_TOKEN") or settings.get("access_token", "")
    client = Client(endpoint, token)

    if args.command == "bot" and args.subcommand == "list":
        values = parse.urlencode({k: v for k, v in {"channel": args.channel, "role": args.role}.items() if v})
        path = "/api/v1/bots" + (f"?{values}" if values else "")
        print_result(client.request_json("GET", path), args.output)
        return 0

    if args.command == "bot" and args.subcommand == "create":
        payload = {
            "id": args.id,
            "name": args.name,
            "description": args.description,
            "role": args.role,
            "channel": args.channel,
            "model_id": args.model_id,
        }
        print_result(client.request_json("POST", "/api/v1/bots", payload), args.output)
        return 0

    if args.command == "member" and args.subcommand == "list":
        print_result(client.request_json("GET", room_members_path(args.channel, args.room_id)), args.output)
        return 0

    if args.command == "member" and args.subcommand == "create":
        payload = {
            "room_id": args.room_id,
            "inviter_id": args.inviter_id,
            "user_ids": [args.user_id],
            "locale": args.locale,
        }
        print_result(client.request_json("POST", room_members_path(args.channel, args.room_id), payload), args.output)
        return 0

    if args.command == "room" and args.subcommand == "create":
        participants = [part.strip() for part in args.participant_ids.split(",") if part.strip()]
        payload = {
            "title": args.title,
            "description": args.description,
            "creator_id": args.creator_id,
            "participant_ids": participants,
            "locale": args.locale,
        }
        print_result(client.request_json("POST", channel_path(args.channel, "rooms"), payload), args.output)
        return 0

    if args.command == "message" and args.subcommand == "create":
        payload = {
            "room_id": args.room_id,
            "sender_id": args.sender_id,
            "mention_id": args.mention_id,
            "content": args.content,
        }
        print_result(client.request_json("POST", channel_path(args.channel, "messages"), payload), args.output)
        return 0

    parser.error("unsupported command")
    return 2


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
