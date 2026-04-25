# Internal KPI SeaTalk Bot

Lightweight Go server that polls `Internal_kpi!S15:T39` every 5 minutes. When values change, it waits 7 seconds, exports `Internal_kpi!G1:U39` as a PDF, renders it with Poppler and ImageMagick, then posts a SeaTalk interactive card plus the captured KPI image to every known group.

## Requirements

- SeaTalk app with bot capability, event callback, and group message permission enabled.
- Google service account with access to spreadsheet `1pLN46ZKWJIsidswMeoxhZwoacuFMR08sCaTFG6mLytc`.
- The spreadsheet must have:
  - `Internal_kpi`
  - `bot_config`
  - group IDs stored in `bot_config!A2:A`

## Configure

Copy `.env.example` to `.env` and fill in:

```env
SEATALK_APP_ID=
SEATALK_APP_SECRET=
SEATALK_SIGNING_SECRET=
GOOGLE_APPLICATION_CREDENTIALS=/run/secrets/google-service-account.json
```

Place the Google service account JSON at `./service-account.json`, or change the Docker Compose volume.

Set the SeaTalk callback URL to:

```text
https://your-public-host/seatalk/callback
```

SeaTalk callback verification is handled by the server.

## Sheet Polling

The server polls Google Sheets directly. Defaults:

```env
ENABLE_SHEET_POLLING=true
POLL_INTERVAL=5m
SETTLE_INTERVAL=7s
```

On startup, the server reads the watch range once to establish a baseline. Every 5 minutes after that, if the watched values differ from the last seen values, it schedules capture after `SETTLE_INTERVAL`.

Image render defaults:

```env
PNG_DPI=180
PNG_MAX_WIDTH=1600
```

## Run

```bash
docker compose up --build
```

Health check:

```text
GET /healthz
```

## Deploy On Render

This repo includes [render.yaml](render.yaml) for a Docker web service. On Render, create the service from the repo blueprint or create a Docker web service manually.

Set these secret environment variables in Render:

```env
SEATALK_APP_ID=
SEATALK_APP_SECRET=
SEATALK_SIGNING_SECRET=
GOOGLE_CREDENTIALS_JSON=
```

For `GOOGLE_CREDENTIALS_JSON`, paste the full Google service account JSON as the environment value. You do not need `GOOGLE_APPLICATION_CREDENTIALS` on Render.

After deployment, use the Render service URL:

```text
https://your-render-service.onrender.com/healthz
https://your-render-service.onrender.com/seatalk/callback
```

Set the SeaTalk callback URL to `/seatalk/callback`.

## Group ID Handling

When the bot is added to a SeaTalk group, the callback handler stores the `group_id` in `bot_config!A2:A`. When the bot is removed, it removes that ID. A daily sync normalizes the sheet list by sorting and deduplicating known IDs.

The local SeaTalk docs in this repo do not include an API for listing all groups the bot has joined. Because of that, the server can only sync groups it has learned from callback events or IDs already present in `bot_config!A2:A`.
