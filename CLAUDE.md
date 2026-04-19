# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
# Build
go build -o findsenryu -ldflags '-s -w' .

# Run
./findsenryu

# Dependencies
go mod tidy
```

For deployment (Linux/systemd), use `rebuild.ps1` or `scripts/setup-systemd.ps1`.

## Environment Setup

Copy `sample.env` to `.env` and fill in values:

| Variable | Required | Purpose |
|----------|----------|---------|
| `DISCORD_TOKEN` | Yes | Bot token |
| `CDN_UPLOAD_URL` | No | CDN endpoint for image uploads |
| `CDN_TOKEN` | No | Bearer token for CDN auth |
| `QUOTE_API_URL` | No | External image generation API |
| `DISCORD_PLAYING` | No | Bot status text |

Data is stored in `data/senryu.db` (SQLite3) and `data/ledis/` (LedisDB), both auto-created on startup.

## Architecture

A Discord bot that detects Japanese senryu (5-7-5 mora poems) in messages, stores them, and generates images.

### Key files

- **`main.go`** — Discord event handlers (`messageCreate`, `interactionCreate`), slash command registration and routing. All Discord-facing logic lives here.
- **`miq.go`** — Senryu-to-image pipeline: calls `QUOTE_API_URL` to render a poem, uploads the result to CDN, returns the URL to Discord. Handles avatar caching and opt-out checks.
- **`config/config.go`** — Loads `.env` via `godotenv`, exposes a `Config` struct.
- **`db/db.go`** — Initializes SQLite with GORM and auto-migrates models (`Senryu`, `YomeMessage`, `AvatarCache`).
- **`db/ledis.go`** — Initializes LedisDB for fast set operations (muted channels, opted-out users).
- **`service/`** — Business logic: `senryu.go` (CRUD + ranking), `mute.go` (channel mute via LedisDB), `optout.go` (user opt-out toggle), `yome_message.go` (grouped author tracking).
- **`model/`** — Data structs: `Senryu`, `AvatarCache`, `YomeMessage`.

### Data flow

1. `messageCreate` in `main.go` calls `go-haiku` to detect 5-7-5 patterns.
2. Detected senryu → `service.CreateSenryu()` → SQLite row + LedisDB sorted set (for ranking).
3. `/rank` reads the LedisDB sorted set; `/川柳を画像化` context menu triggers `miq.go`.
4. `miq.go` fetches a random opted-in author's avatar, POSTs to `QUOTE_API_URL`, then PUTs the image to `CDN_UPLOAD_URL`.

### Key dependencies

- `github.com/darui3018823/discordgo` — patched fork of discordgo (not upstream)
- `github.com/0x307e/go-haiku` — 5-7-5 syllable detection
- `github.com/jinzhu/gorm` + `mattn/go-sqlite3` — SQLite ORM
- `github.com/ledisdb/ledisdb` — fast key-value/set store for mutes and opt-outs
