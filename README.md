# Internal KPI SeaTalk Bot

Lightweight Go server that receives change signals for `Internal_kpi!S15:T39`. When values change and remain stable for 7 seconds, it exports `Internal_kpi!G1:U39` as a PDF, renders it with Poppler and ImageMagick, then posts a SeaTalk interactive card plus the captured KPI image to every known group.

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
KPI_WEBHOOK_SECRET=
GOOGLE_APPLICATION_CREDENTIALS=/run/secrets/google-service-account.json
```

Place the Google service account JSON at `./service-account.json`, or change the Docker Compose volume.

Set the SeaTalk callback URL to:

```text
https://your-public-host/seatalk/callback
```

SeaTalk callback verification is handled by the server.

## Apps Script Polling

Use [appscript/InternalKPIWatcher.gs](appscript/InternalKPIWatcher.gs) as a bound Apps Script on the spreadsheet. It checks `Internal_kpi!S15:T39` on a time-driven trigger, stores a hash of the watched values, and calls the Go server only when the hash changes.

In Apps Script, run once:

```javascript
configureInternalKpiWatcher(
  'https://your-public-host/kpi/change',
  'same-value-as-KPI_WEBHOOK_SECRET'
);
installInternalKpiWatcher();
```

Apps Script time-driven triggers are minute-level polling, not immediate cell-change events. After Apps Script detects a change, the Go server waits `SETTLE_INTERVAL`, default `7s`, before capturing and sending.

The Go server also has built-in polling available for fallback:

```env
ENABLE_SHEET_POLLING=true
```

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

## Group ID Handling

When the bot is added to a SeaTalk group, the callback handler stores the `group_id` in `bot_config!A2:A`. When the bot is removed, it removes that ID. A daily sync normalizes the sheet list by sorting and deduplicating known IDs.

The local SeaTalk docs in this repo do not include an API for listing all groups the bot has joined. Because of that, the server can only sync groups it has learned from callback events or IDs already present in `bot_config!A2:A`.
