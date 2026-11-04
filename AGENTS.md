# Working on Komanook

Komanook is an invitation-only manga reader and reader-identity pilot. One Go application, PostgreSQL, code-owned templ views, and small browser modules. Read [docs/architecture.md](docs/architecture.md) before changing authentication, data boundaries, jobs, deployment or publication. Read [docs/design-system.md](docs/design-system.md) before changing UI, CSS, component controllers or DOM patch targets. Read [CONTEXT.md](CONTEXT.md) before changing reader identity, Work/Edition relationships, or profile showcases. [docs/research.md](docs/research.md) records the source evidence behind dependency choices; [docs/roadmap.md](docs/roadmap.md) records deliberate scope limits.

## Invariants

- Publish only explicitly authorized/licensed content. Original CC0 fixtures have a rights record in `content/LICENSE.md`. Upload acceptance never implies publication.
- SCS owns session mechanics; nosurf owns CSRF; bcrypt owns password hashing; River owns durable jobs. Extend through their APIs, keep tokens out of logs, and retain server-side permission checks.
- Resolve identity from the session and reload account state/version/role from PostgreSQL. Scope personal data to that identity. Treat database failures as denial/unavailability.
- Keep guest reading progress and per-account local progress in separate versioned storage keys. The account-key suffix only partitions browser storage; it is not an authorization token. Guest import must be an explicit CSRF-protected action, transactional, bounded, and insert-only when an account bookmark already exists.
- Public cached catalog/series HTML has no identity, CSRF token or Set-Cookie. Reader profile pages use `private, no-store` so a visibility change takes effect without an edge-cache delay. Private responses use `no-store`. Preserve `Vary: Datastar-Request` for full-page/fragment responses.
- Public reader profiles are opt-in and separate from login accounts. Public profile handlers use only the session-independent public query, expose no email or chapter-level progress, and include aggregate reading milestones only when the profile enables them. Owner preview uses the same public layout behind authentication and `no-store`. Writes derive the owner from the authenticated session.
- Favorite manga and private library entries reference canonical Work IDs independently of hosted-edition publication. Library status and rating are scoped to the signed-in account and create no public activity. The current hosted `series` record is an edition; keep chapter and page progress edition-scoped and add new editions without changing a Work's identity.
- Follow the DOM ownership table. Patch only `#catalog-results`; preserve the form/focus, MannerHTML root and reader. Reader JS has no Datastar or component-runtime imports.
- Keep SQL in `db/queries` and regenerate sqlc output. Keep view source in `.templ` and regenerate `_templ.go`. Commit generated code, lockfiles, and built assets. Review generated diffs alongside source changes. Routine `make generate assets` must be stable across operating systems; use `npm run art` only after intentional story/art edits, then inspect the rendered images before committing.
- Tailwind source detection is opt-in: `internal/web/styles.css` disables automatic scanning and explicitly registers `internal/web/*.templ`. Keep utility-class source in those templates or register any new source path explicitly; CSS and generated code are not scan inputs.
- Use explicit dependencies and domain-specific functions. Add an interface when there is a real alternate implementation or test seam. Keep the original art, application code and third-party license scopes distinct.

## Work loop and completion

1. Inspect the relevant source and tests. Preserve unrelated changes.
2. Implement the smallest complete behavior, including its failure response and native browser fallback.
3. Run `make generate assets` after SQL/template/style changes; after story/art source changes also run `npm run art` and inspect the output. Run `gofmt` for Go and `npm run format` for owned JS. Vendored library code is excluded from the formatter.
4. Run `make check`, relevant Go tests, and `make build`. Security, SQL, session or transaction changes require the real PostgreSQL tests via `TEST_DATABASE_URL` (plain `go test` skips them when absent).
5. UI/reader/profile/DOM ownership changes require `npm run test:browser` against a seeded server. Inspect desktop/mobile screenshots. Reader performance changes require `npm run bench`, including the 250-page fixture; report environment and limits from `docs/performance.md`.

`npm run dev:local` supplies a loopback-only PostgreSQL and seeded local accounts without Docker. `npm run test:local` supplies an isolated cluster, race tests, browser tests and benchmarks; stop the dev server first because both use port 8080. See [README.md](README.md) for setup, credentials and external-database commands. Use a test PostgreSQL role with CREATEDB; integration tests create/drop only randomly named `komanook_test_*` databases.

Completion means the relevant checks passed, generated output is current, rights/provenance is retained, and limitations are stated. Never report an inspected upstream test as one you executed, a local benchmark as field performance, or automated axe as full accessibility conformance. Account recovery, public registration, object storage and chapter publication are deferred capabilities, not active application paths.
