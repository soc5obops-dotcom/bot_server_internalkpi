# Internal KPI SeaTalk Bot

Lightweight Go server that exports `revamped_bot_server!F1:AD59` as a PDF on a fixed schedule, renders it with Poppler and ImageMagick, then posts a SeaTalk interactive card plus the captured KPI image to every known group.

## Requirements

- SeaTalk app with bot capability, event callback, and group message permission enabled.
- Google service account with access to spreadsheet `1pLN46ZKWJIsidswMeoxhZwoacuFMR08sCaTFG6mLytc`.
- The spreadsheet must have:
  - `revamped_bot_server`
  - `bot_config`
  - group IDs stored in `bot_config!A2:A`

## Configure

Copy `.env.example` to `.env` and fill in:

```env
SEATALK_APP_ID=
SEATALK_APP_SECRET=
SEATALK_SIGNING_SECRET=
ADMIN_TOKEN=
GOOGLE_APPLICATION_CREDENTIALS=/run/secrets/google-service-account.json
```

Place the Google service account JSON at `./service-account.json`, or change the Docker Compose volume.

Set the SeaTalk callback URL to:

```text
https://your-public-host/seatalk/callback
```

SeaTalk callback verification is handled by the server.

## Scheduled Sends

The server sends the report on a fixed schedule. Value changes in the watch range do not trigger report sends.

```env
ENABLE_SCHEDULED_SENDS=true
```

The bot sends the report every day at 6AM, 10AM, 1PM, 3PM, 6PM, 9PM, 12MN, and 4AM in `APP_TIMEZONE`.

Image render defaults:

```env
PNG_DPI=300
PNG_MAX_WIDTH=2400
```

The renderer will retry narrower output sizes if the encoded image would exceed SeaTalk's 5 MB image limit.

## Run

```bash
docker compose up --build
```

Health check:

```text
GET /healthz
```

Manual report test, enabled only when `ADMIN_TOKEN` is set:

```bash
curl -X POST https://bot-server-internalkpi.onrender.com/admin/test-report \
  -H "Authorization: Bearer your-admin-token"
```

This immediately captures the report and sends the SeaTalk interactive card plus image to group IDs in `bot_config!A2:A`.

## Deploy On Render

This repo includes [render.yaml](render.yaml) for a Docker web service. On Render, create the service from the repo blueprint or create a Docker web service manually.

Set these secret environment variables in Render:

```env
SEATALK_APP_ID=
SEATALK_APP_SECRET=
SEATALK_SIGNING_SECRET=
ADMIN_TOKEN=
GOOGLE_CREDENTIALS_JSON=
```

For `GOOGLE_CREDENTIALS_JSON`, paste the full Google service account JSON as the environment value. You do not need `GOOGLE_APPLICATION_CREDENTIALS` on Render.

After deployment, use the Render service URL:

```text
https://your-render-service.onrender.com/healthz
https://your-render-service.onrender.com/seatalk/callback
https://your-render-service.onrender.com/admin/test-report
```

Set the SeaTalk callback URL to `/seatalk/callback`.

## Group ID Handling

When the bot is added to a SeaTalk group, the callback handler stores the `group_id` in `bot_config!A2:A`. When the bot is removed, it removes that ID. A daily sync normalizes the sheet list by sorting and deduplicating known IDs.

The local SeaTalk docs in this repo do not include an API for listing all groups the bot has joined. Because of that, the server can only sync groups it has learned from callback events or IDs already present in `bot_config!A2:A`.
