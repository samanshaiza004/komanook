# Komanook

A small Go foundation for an independent manga, manhwa and webcomic reading platform. Server-rendered catalog and series pages, an independent reader, real PostgreSQL account progress, invitation-only login, private image normalization, and a MangaDex API read-through for community scanlations. Komanook does not scrape or store MangaDex chapter images. No SPA, Redis or hosted identity service is required to run the slice.

![Komanook catalog](docs/screenshots/desktop-catalog.png)

## Run locally

Requires **Go 1.27.3**, **Node 26.13.0** (Node 24+ is supported by the build tools), and macOS or Linux. Exact dependencies are locked. The helper downloads a development-only PostgreSQL binary through npm; it binds to loopback and never creates a system user. Run as your normal user.

```sh
npm ci
npm run dev:local
```

Open [Komanook locally](http://localhost:8080). The helper initializes a persistent development database under `.data/postgres`, migrates it, seeds three original four-page comics, provisions fixture accounts, builds the app and starts it. Stop with Ctrl+C. Application logs: `.data/server.log`.

| Local fixture account | Password | Permission |
| --- | --- | --- |
| `reader@komanook.test` | `a quiet place to read` | Read and save personal progress |
| `operator@komanook.test` | `a quiet place to work` | Also submit private image-processing jobs |

These accounts exist only because the local helper explicitly provisions them. The production app has no built-in accounts or passwords. Restarting the local helper resets fixture passwords and revokes their previous sessions. Read all samples without signing in; guest progress stays in a separate device namespace. Signed-in progress has its own per-account device namespace and is also saved to PostgreSQL. On the account page, you can explicitly import guest bookmarks; existing account bookmarks are kept.

## MangaDex community chapters

Use the separate MangaDex search from the catalog to browse its community scanlation data. MangaDex chapter images are served from MangaDex@Home when a reader opens a chapter; Komanook does not copy or persist those image files. Each chapter page credits MangaDex and its scanlation group. The MangaDex API client uses a request cap and short-lived in-memory delivery metadata cache; it does not retry rate-limited requests.

MangaDex's [API acceptable-use policy](https://api.mangadex.org/docs/redoc.html#section/Acceptable-use-policy) requires credit to MangaDex and scanlation groups, honoring written removal requests from active group leaders, and no ads or paid services anywhere on the website or app while its API is used. Komanook stays free and ad-free while the MangaDex read-through is active. Set `MANGADEX_REMOVAL_CONTACT` to a written contact route (a `mailto:` address or HTTPS URL) before production; production startup fails without it. After verifying a written request against the group's active MangaDex page, an administrator can record the group ID and request reference under **Your reading space → Scanlation group removal requests**; chapter lists and reader routes then hide all chapters associated with that group.

## Use an existing PostgreSQL or Docker

`compose.yaml` supplies only a local database with pinned image digest. Start it with `docker compose up -d --wait`, then set the following variables in your shell. `.env.example` is documentation; the app does not implicitly load `.env` files.

```sh
export APP_ENV=development
export HTTP_ADDR=127.0.0.1:8080
export BASE_URL=http://localhost:8080
export DATABASE_URL='postgres://komanook:local-komanook-only@localhost:5432/komanook?sslmode=disable'
export MEDIA_DIR=.data/media
make build
./bin/komanook migrate
./bin/komanook seed
./bin/komanook user --email you@example.com --role reader
./bin/komanook serve
```

`user` reads a hidden password interactively or one line from stdin. It validates and bcrypt-hashes 12–72 bytes, and invalidates previous sessions when updating an account. Use `--role admin` for the private image tools on the account page. There is no public signup/recovery workflow yet. To disable an account immediately, set `users.disabled=true`; protected requests reload that state.

## Develop and verify

```sh
make generate assets       # pinned sqlc/templ; vendored browser assets and Tailwind
                           # run `npm run art` after intentional story/art edits
make check                 # format check, go vet, owned-JS syntax/format
make build                 # production binary; no Node runtime needed
npx playwright install chromium
npm run test:local         # isolated real PostgreSQL + race + desktop/mobile + benchmark
```

Stop `dev:local` before `test:local`, or set `KOMANOOK_TEST_HTTP_PORT=8081` to run the isolated test app beside it. The test helper uses PostgreSQL port 54330, creates a temporary cluster, and removes it on exit. Chromium installation is separate and pinned to the Playwright lockfile. Docker is optional for local verification. CI uses a PostgreSQL service and also validates the container build. Tailwind scans only explicitly registered template sources, so the committed CSS can be regenerated identically across operating systems.

With a running seeded app and your own test database:

```sh
export TEST_DATABASE_URL='postgres://test_user:test_password@localhost:5432/test_database?sslmode=disable'
go test -race ./...
npm run test:browser
npm run bench
```

The test role needs CREATEDB: tests create/drop isolated, randomly named `komanook_test_*` databases. **Without `TEST_DATABASE_URL`, real database tests skip explicitly**; this is not full verification. Browser checks cover desktop/mobile widths, keyboard LTR/RTL, vertical progress, guest/account isolation and import, blocked storage, failed sync, a 250-page reader fixture, native no-JS paths, MannerHTML after Datastar patches, and automated axe checks. Integration tests cover session renewal/revocation/expiry, role checks, CSRF, escaping, non-overwriting transactional import, rollback and storage failures. Raw measurements, method and limits are in [docs/performance.md](docs/performance.md).

## Production build and deployment

`make build` uses committed generated assets. To regenerate from source first, use `npm ci && make generate assets build`. The separately invoked `npm run art` rasterizes the original CC0 story artwork with the host's available sans-serif font; inspect changed covers/pages before committing. Routine generation and CI preserve those committed WebP files so font differences across operating systems cannot cause asset drift. Browser assets are self-hosted at URLs fingerprinted from their bytes. PostgreSQL migrations are embedded but run only through the explicit `migrate` command. The app refuses startup with missing/dirty migrations.

```sh
docker build -t komanook:foundation .
```

The Dockerfile pins build/runtime image digests, produces a static binary, runs as UID 65532, and includes third-party notices. Deploy one app container and PostgreSQL behind a TLS reverse proxy. Configure:

- `APP_ENV=production`, `HTTP_ADDR=0.0.0.0:8080`, `BASE_URL=https://your-domain`.
- `MANGADEX_REMOVAL_CONTACT=mailto:you@your-domain` or an HTTPS contact page, so active scanlation group leaders have a written removal route.
- `DATABASE_URL` with `sslmode=verify-full` and the provider's trusted CA (`sslrootcert` when needed). Use separate migration/operator and least-privilege runtime credentials.
- `MEDIA_DIR=/var/lib/komanook` on a private persistent volume writable by UID 65532. It is never served publicly. Start with a memory limit around 512 MiB and observe actual worker peaks; two bounded image workers share the process.

Run the image once with command `migrate` before starting `serve`, using release credentials. Provision your own account through `user`; the sample `seed` command is intentionally disabled in production. Use a staging environment for the bundled sample pilot. `/health/live` checks the process; `/health/ready` checks database and migration readiness. Set a proxy request-body limit of 11 MiB, enforce HTTPS/Host, preserve the external Host header, and avoid caching account/reader/API/admin responses. Cache anonymous catalog/series pages according to their headers and preserve the Datastar request variant. Compression belongs at the proxy/CDN. Logs omit request bodies, passwords, session tokens, email addresses and search strings.

Login IP budgets use the direct peer address and ignore forwarding headers. Behind one proxy, that is a shared budget; configure edge abuse control before a public pilot and introduce trusted-proxy IP handling only with explicit trust configuration. Do not trust arbitrary client IP headers.

The private admin upload action returns a job ID and status URL. Poll `GET /admin/uploads/{id}` while signed in as an administrator. Input is quarantined and validated; River transactionally schedules bounded JPEG/PNG normalization and records private derivatives. There is no automatic public image URL or local chapter publication endpoint. Remove abandoned quarantine files only after reconciling upload/job state; add a retention task before sustained ingestion. A real R2 adapter, atomic local chapter assembly/publication, rights revocation/purge, and public account lifecycle are prioritized next.

## Decisions, provenance and limits

- [Architecture and security boundaries](docs/architecture.md)
- [Source-based foundation comparison](docs/research.md)
- [Design tokens and DOM ownership](docs/design-system.md)
- [Prioritized roadmap](docs/roadmap.md)
- [Performance evidence](docs/performance.md)
- [Agent conventions](AGENTS.md)
- [Original CC0 sample rights](content/LICENSE.md)
- [Third-party notices and browser provenance](licenses/)

Selected: independent Go/templ/sqlc/pgx composition, SCS/nosurf/bcrypt, River, compiled Tailwind v4, one shadcn-derived native button, MannerHTML tabs and narrowly scoped Datastar. Rejected wholesale: goilerplate and go-starter's broader SaaS scaffolds, Datapages' framework/session conventions, and a second component runtime. The research records concrete reasons and current-source evidence, including goilerplate's free v4 license.

This is a tested foundation for a private pilot. It is not a public consumer launch: registration/recovery/email, production rights operations, backups/restore drills, R2/CDN setup, long-chapter native image-memory profiling, mobile assistive-technology validation and independent security review remain. Datastar's evaluator currently requires `unsafe-eval` on catalog/series; the reader and account pages have a stricter policy. No advertising, billing, marketplace, moderation suite or creator payouts are implemented.
