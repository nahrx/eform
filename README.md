# eForm

A self-hosted platform for building digital questionnaires, sharing them by link, and
collecting responses — built for field data collection where the network is unreliable.

Instruments are designed in a drag-and-drop builder, published, and handed to
respondents as a link. Respondents sign in with Google, fill the form in a browser that
keeps working offline, and their answers sync when a signal comes back.

---

## Contents

- [Features](#features)
- [Who can do what](#who-can-do-what)
- [Stack](#stack)
- [Getting started](#getting-started)
- [Configuration](#configuration)
- [Make targets](#make-targets)
- [Project layout](#project-layout)
- [HTTP API](#http-api)
- [Offline mode](#offline-mode)
- [Backup and restore](#backup-and-restore)

---

## Features

**Form builder.** Pages, blocks, sections, and rosters (repeating groups, inline or on
their own subpage), with 26 field types:

| Category | Types |
|---|---|
| Input | text, email, textarea, number, integer, decimal, currency, range, rating, calculated, hidden |
| Choice | select, multiselect, radio, checkbox, boolean |
| Date & time | date, time, datetime |
| Media | geopoint, photo, file, signature, barcode |
| Structure | markdown, note |

Choice options can be typed by hand, drawn from an inline reference table, or fetched
from an external API with cascading filters. Fields support conditional visibility,
enablement, and requiredness, skip logic, custom validation expressions, and calculated
values.

**Sharing.** Per-form share links with an optional password, expiry date, and a
restricted mode that only lets listed email addresses in.

**Collection.** Responses save as drafts while being filled and can be exported to CSV
or XLSX. Editors' changes to submitted answers are recorded as revisions.

**Offline first.** Multi-response forms install as a PWA. Answers and photos queue in
IndexedDB while offline and flush when the connection returns; devices report what they
are still holding so admins can chase stranded data.

**Photo compression.** Photo fields shrink images in the browser before upload, to a
per-field size budget that defaults to 200 KB. It can be switched off per field.

**Public read-only API.** External systems can pull responses with a scoped API key —
see [HTTP API](#http-api).

**Audit trail.** Admin actions are logged, sessions can be revoked, and write endpoints
are rate-limited per respondent and per IP.

---

## Who can do what

| Role | Scope |
|---|---|
| `superadmin` | Everything, including user management |
| `admin` | Owns forms: build, publish, share, edit and delete responses, manage API keys |
| `editor` | Reads and corrects responses on forms they are assigned to |
| `viewer` | Reads and exports responses on forms they are assigned to |
| Respondent | Signs in with Google; fills forms and sees only their own responses |

Viewer and editor permissions are granted per form, and can be narrowed further to a
named list of respondents. One account can be a viewer on one form and an editor on
another.

---

## Stack

Go 1.23 on the server, PostgreSQL 13+ for storage, and vanilla JavaScript on the
front end — no framework, no bundler, no build step for the UI.

Five direct dependencies: `pgx` (Postgres), `golang-jwt` (tokens), `godotenv` (config),
`x/crypto` (password hashing), `x/image` (image handling). Routing uses the standard
library's `net/http` mux. XLSX export is written by hand rather than pulled in as a
dependency.

Database migrations live in `migrations/` and run automatically at startup — each in its
own transaction, tracked in a `schema_migrations` table.

---

## Getting started

### With Docker Compose

```bash
cp .env.example .env
docker compose up -d
```

This starts PostgreSQL 16, runs the migrations, and serves the app on `APP_PORT` — 8080
if you copied `.env.example`, otherwise Compose falls back to 8789. Once the app reports
healthy, a one-shot `seeder` container loads the Indonesian region reference data and
exits.

### Locally

Requires Go 1.23+ and a reachable PostgreSQL 13 or newer.

```bash
cp .env.example .env
make db-create
make dev
```

Then open <http://localhost:8080>. Sign in with the `SUPERADMIN_*` credentials from
your `.env` — that account is created only while the users table is still empty.

**Change the superadmin password after the first login.**

To load the region reference data used by cascading region dropdowns:

```bash
make seed
```

---

## Configuration

Configuration is read from the environment, or from `.env` in the working directory.
Point `ENV_FILE` somewhere else to override the path. See `.env.example` for the
annotated full list.

| Variable | Default | Notes |
|---|---|---|
| `PORT` | `8080` | HTTP listen port |
| `POSTGRES_HOST` / `PORT` / `USER` / `PASSWORD` / `DB` / `SSLMODE` | — | Individual database settings |
| `DATABASE_URL` | — | Full connection string; overrides the `POSTGRES_*` values |
| `JWT_SECRET` | random | **Set this in production.** Left empty, every token dies on restart |
| `JWT_RESPONDENT_SECRET` | random | Same, for respondent tokens |
| `JWT_TTL_HOURS` | `24` | Token lifetime |
| `CORS_ORIGINS` | `*` | Comma-separated allowed origins |
| `PUBLIC_BASE_URL` | `http://localhost:8080` | Used to build share links |
| `WEB_DIR` | `web` | Application pages and assets |
| `PUBLIC_DIR` | `public` | Landing page served at `/` |
| `ORGANISATION_NAME` | — | Substituted into page titles, landing page, and form footers |
| `ORGANISATION_NICKNAME` | — | Short form, used where space is tight |
| `SUPERADMIN_USERNAME` / `EMAIL` / `PASSWORD` | — | Bootstrap account, created once |
| `GOOGLE_CLIENT_ID` / `SECRET` / `REDIRECT_URL` | — | Respondent and viewer sign-in |

Organisation names are substituted when a page is served, so changing them means
restarting the server — no HTML needs editing.

---

## Make targets

```
make help        List the available targets
make dev         Run without producing a build artifact
make build       Compile to bin/
make run         Build, then run
make test        Run the unit tests
make vet         go vet
make seed        Load region data from data/wilayah_indonesia.csv
make db-create   Create the local database
make db-drop     Drop the local database — DESTRUCTIVE
make db-backup   Dump to backups/eform-<timestamp>.dump
make db-restore  Restore: make db-restore FILE=backups/xxx.dump — OVERWRITES data
```

---

## Project layout

```
main.go                  entry point: config, pool, migrations, server
internal/
  config/                environment parsing
  db/                    connection pool and the migration runner
  models/                domain types
  store/                 all SQL; permission scoping lives here
  auth/                  password hashing, JWT issue and verify
  httpapi/               routing, middleware, handlers
migrations/              numbered .up.sql files, applied at startup
web/                     application pages and assets (no build step)
public/                  landing page served at "/"
cmd/seeder/              region reference-data loader
data/                    region CSV
```

Front-end code worth knowing about, all plain scripts shared between pages:

| File | Purpose |
|---|---|
| `web/builder.js` | The form builder and its live preview |
| `web/public.html` | Form filling, drafts, offline queue, uploads |
| `web/offline-queue.js` | IndexedDB queue, shared with the service worker |
| `web/image-compress.js` | In-browser photo compression |
| `web/geo-map.js` | Leaflet map for geopoint fields |
| `web/sw.js` | Service worker: precache and flush |
| `web/i18n.js` | English source text, translated to Indonesian at runtime |

---

## HTTP API

Two separate surfaces.

**Internal API** (`/api/…`) backs the web UI. It authenticates with a JWT from
`POST /api/auth/login` and enforces the roles above.

**Public API** (`/api/v1/…`) is for external systems. It authenticates with a per-form
API key, is **read-only by design**, and is scoped by the key itself — active flag,
expiry, IP allowlist, request quota, and which respondents' data it may see.

```
GET /api/v1/me
GET /api/v1/forms/{formId}/responses
GET /api/v1/forms/{formId}/responses.csv
GET /api/v1/forms/{formId}/responses/{responseId}
```

Keys are created, rotated, scoped, and revoked from a form's management page, which
also shows the key's request log.

`GET /healthz` returns `{"status":"ok"}` for health checks.

---

## Offline mode

Multi-response forms register a service worker and install as a PWA. While offline:

- Answers are queued in IndexedDB and submitted when the connection returns.
- Photos are stored as local blobs behind a placeholder reference, then uploaded and
  the reference rewritten once the upload succeeds.
- A response the server rejects is set aside for review rather than blocking the rest
  of the queue.
- Devices report their queue state, so `GET /api/forms/{id}/queue-reports` shows admins
  which devices are still holding unsent data.

---

## Backup and restore

```bash
make db-backup
make db-restore FILE=backups/eform-20260101-120000.dump
```

Backups use `pg_dump --format=custom`. Restore passes `--clean --if-exists`, so it
**overwrites the target database**.

Uploaded files live outside the database, under `<PUBLIC_DIR>/uploads` — mounted as the
`uploads_data` volume in the Compose setup. Back that up alongside the dump; a database
restore on its own will leave every photo and attachment pointing at nothing.
