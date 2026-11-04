# Komanook architecture

Decision: 2026-11-03. Status: accepted for the foundation. Evidence and source snapshots: [research.md](research.md).

## Small, independent composition

One Go modular monolith, one PostgreSQL database, one deployable binary. Use `net/http` routing, patched templ, sqlc/pgx, SCS with its maintained PostgreSQL store, nosurf, and River's PostgreSQL queue. Compile Tailwind v4 at build time. Adopt shadcn-templ's source-ownership approach for a small primitive; use MannerHTML for tabs and native HTML for everything else. Datastar enhances bounded catalog results; the reader is a separate browser module. No application framework, SPA, Redis, or browser hydration.

The existing repository was empty. No inherited application, migrations, tests, or license needed preserving. The reference audit is deliberately bounded; source inspection is not a security audit of those projects.

## Boundaries and dependencies

| Boundary | Owns | Dependencies |
| --- | --- | --- |
| `internal/config` | Validated environment, production constraints | standard library |
| `internal/database` + `db` | pool, migrations, generated queries, seed transaction | pgx, migrate, sqlc output |
| `internal/auth` | password verification/provisioning policy | bcrypt; HTTP composes SCS/nosurf |
| `internal/media` | quarantined uploads, bounded image normalization, River worker | database, filesystem, image codecs |
| `internal/web` | routes, authorization, caching, view models, templates, assets | explicit concrete services above |
| `internal/web/static/reader.js` | reading mode, keyboard controls, image window, local progress | browser APIs; small JSON progress contract |
| `cmd/komanook` | composition, lifecycle, operator commands | all boundaries |

SQL lives in `db/queries`, not template handlers. sqlc's generated queries are a data-access boundary, not a generic repository. Introduce domain services when multi-step rules need them; do not add empty service/interface layers for CRUD. Public catalog pages never load a session or emit a CSRF cookie, so the CDN may cache them without leaking account data. Account navigation is a generic link to a private account page. Search and filter inputs are bounded and parameterized. Public series and chapters must be published and have recorded rights.

## Security and account scope

Provisioned accounts only in this slice. Operator CLI hashes passwords using `golang.org/x/crypto/bcrypt`, cost 12, accepts 12–72 UTF-8 bytes, and never stores cleartext. Login has a generic failure response and performs the same-cost dummy comparison for unknown accounts. A PostgreSQL-backed account/IP attempt budget bounds work across processes; a small concurrent verifier limit bounds CPU. User identifiers come from the session, never request JSON. Roles and disabled state are reloaded from the database on every protected request. Permission checks precede uploads.

SCS owns opaque random session tokens, persistence, idle/absolute expiration, renewal on login, and destruction on logout. Session tokens are hashed by SCS before storage. Use host-only HttpOnly SameSite=Lax cookies, Secure and `__Host-` names in production. nosurf checks masked tokens and origin for every unsafe browser request, including login/logout/progress/uploads. No exemptions. Configure the external HTTPS origin explicitly; never trust arbitrary forwarding headers. Application errors fail closed. Private HTML/JSON uses `private, no-store`; CSRF-bearing HTML never enters the public cache. templ escapes content; no untrusted raw HTML or SVG. A restrictive CSP permits Datastar's expression evaluator (`unsafe-eval`); this is a recorded tradeoff, not an omitted CSP. Production HSTS is enabled only in production.

Public registration, password reset, email changes, MFA and federation are deferred. This is a real invitation-only login foundation, not a complete consumer identity product. Before public signup, adopt an established OIDC identity service or add vetted lifecycle workflows and transactional email, verification, abuse controls, and recovery tests. Do not improvise token cryptography. Revoke sessions on operator account disable/reset through the account version field; every authenticated session stores that version.

## Reader and progress

Reader HTML contains a crawlable, accessible ordered set of images with dimensions and descriptive transcripts. Without JS it is a continuous strip. With JS: continuous vertical, paginated LTR/RTL, explicit next/previous controls, Arrow keys/Home/End, mode selection, page announcements, and a bounded image window. The first image is eager; later pages use browser lazy loading and async decode. In paginated mode only current/adjacent image sources stay attached. Observe actual visible pages in vertical mode. Controls and text fields keep their native keyboard behavior.

The replaceable reader has no Datastar/MannerHTML dependency and lives outside their patches. Its only server contract is `GET/POST /api/progress/{chapterID}` with an integer page and CSRF header. The server validates chapter publication and page range. Latest received update wins; no client timestamp is trusted. Local storage is bounded, versioned, and optional (blocked storage cannot break reading). On first open, authenticated server progress takes precedence if present; otherwise the local bookmark is used. Network failures keep the bookmark locally and offer retry through subsequent page movement. No claimed offline chapter downloads.

## Media and jobs

Bundled sample stories and illustrations are original Komanook fixtures, explicitly CC0; see `content/LICENSE.md`. There is no scraping or hotlinking. Production R2 delivery is a future adapter, not fake credentials or an unimplemented active code path.

The admin-only upload endpoint accepts a single JPEG/PNG, bounded bytes, dimension and pixel count, ignores supplied filenames, and writes quarantine outside public assets. A metadata insert and River enqueue share one PostgreSQL transaction. A low-concurrency worker decodes and re-encodes to a bounded JPEG, discards metadata, writes an immutable digest name atomically, and records the result. Retries are idempotent. Processed uploads remain private: no public serving or automatic publication in this slice. Chapter assembly, rights review and publication are the next milestone. Failed jobs remain visible in River/database; alert on discarded jobs. Quarantine cleanup and a stronger process/container memory limit are deployment duties; no archive or SVG upload surface exists.

## Data, operations and deployment

Versioned embedded SQL migrations use golang-migrate's PostgreSQL advisory lock; River manages its own schema migrations. Run migrations as an explicit release command, never on every web startup. Use expand/contract migrations and back up before a destructive release. Seed fictional content in a single idempotent transaction. Unique constraints and foreign keys protect relationships. Progress upserts and rate limits are atomic SQL operations.

Build inputs are pinned by Go module sums, npm lockfile, generator versions and vendored browser files with provenance. Generated templ/sqlc output and built assets are committed; CI verifies regeneration. Node is a build/test tool only. The production binary embeds templates and static assets; uploaded files require a persistent private volume until object storage is adopted. Versioned asset URLs are derived from the complete embedded asset tree and cached immutably. HTML uses short explicit public caching only on anonymous catalog/series routes. No personalized CDN caching.

Deploy one non-root container behind a TLS reverse proxy with a managed PostgreSQL database, persistent private media volume, and bounded resources. Two workers initially run inside the same process; River allows a separate process later without a service split. Set request/header/body limits, graceful shutdown, DB pool limits and statement deadlines. `/health/live` is process liveness; `/health/ready` checks DB reachability and migration state. Structured JSON logs record route, status, latency and request ID; omit passwords, tokens, bodies, email addresses and query strings. No unauthenticated pprof or metrics exposing data. External uptime checks plus log-based latency/error/job alerts are sufficient initially. Add OpenTelemetry only when it answers an observed operational question.

Costs are primarily PostgreSQL, compute and image delivery. No numerical cost or speed advantage is asserted without actual traffic measurements. Backups, restore drills, image rights, secret rotation, database TLS, abuse limits, vulnerability scans and retention policy remain release responsibilities. See [roadmap.md](roadmap.md) and the measured performance report.

## Acceptance

Unit and real PostgreSQL tests cover configuration, session rotation/revocation/expiry, role enforcement, CSRF, escaping, private caching, input bounds, transaction rollback, media rejection and normalization, and database failures. Playwright covers keyboard/responsive reading, progress across reloads, no-JS paths, MannerHTML tabs and focus across Datastar updates. A repeatable Chromium desktop/mobile experiment records sizes, loading and heap behavior. Those results describe this sample and this machine, not a competitor comparison or production SLO.
