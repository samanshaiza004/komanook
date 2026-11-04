# Komanook roadmap

## Foundation delivered in this slice

Independent Go/templ app, pinned build tooling, PostgreSQL schema/sqlc queries, explicit migrations, provisioned-account login/logout, SCS sessions, CSRF and role checks, public cached catalog/series pages, original CC0 sample comics, responsive independent reader, local/account progress, a bounded Datastar patch, MannerHTML tabs, private admin image quarantine with transactional River processing, a MangaDex API read-through with scanlation credits and recorded group removals, CI and measured validation.

## 0.1.1: foundation acceptance

The original foundation gates were completed and the latest remote run at `e3dce88` was green: deterministic Tailwind output, isolated guest/account progress, explicit import, a 250-page reader stress fixture, and CI verification. Apply the same gate to reader-profile changes: require a green GitHub Actions run on the exact commit before accepting the iteration. Local results complement but do not replace CI.

## 0.1 identity: first profile slice

Delivered here: private-by-default reader profiles with handles, display names, notes, limited themes, explicit public visibility, up to six ordered Work favorites, and optional aggregate reading milestones. The showcase layout is constrained and versioned. Public pages are crawlable and cacheable without a session; private profile data is not served from public routes.

Next: universal library statuses and chosen editions, public/private curated collections, creator favorites, and taste comparison. Add follows, reviews, comments, and activity only alongside blocking, reporting, moderation, deletion, and spoiler controls. Keep the first profile meaningful without followers or a feed.

## Private content pilot

1. Operator workflow for rights evidence, draft chapter assembly, accessible transcripts, immutable image manifest and atomic publication. Store original source files privately; never publish an upload merely because processing succeeded.
2. Replace local quarantine/derivatives with private R2 buckets, signed upload intent, explicit byte/content limits, bounded processing container, cleanup/retention and separate delivery hostname. Configure CDN cache keys, rights revocation/purge and responsive image variants.
3. Account lifecycle: choose managed OIDC or fully vetted invitation/verification/recovery workflows. Add transactional email only with configured provider, delivery records, retries, suppression and abuse controls. Complete security review before public registration.
4. Database TLS, least-privilege runtime/release roles, backup/restore drill, deploy/rollback drill, secret rotation, operational alerts and ownership. Test worker failure/retry on the chosen deployment environment.
5. Test an actual licensed chapter at hundreds of pages on low-memory phones. Establish production budgets from that measurement, not these tiny fixtures. Accessibility review with screen reader users and native assistive technology.

## Public reading release

Complete account lifecycle, privacy controls, profile/content reporting and moderation, deletion, terms, and rights operations. Add indexed catalog discovery, pagination, structured SEO metadata/sitemap, and PostgreSQL text search before a separate search service. Revisit session duration and identity risk once real usage is known.

Universal tracking follows a canonical Work and retains a chosen Edition; chapter progress stays edition-specific. Add creator identities and ordered work-credit relationships when licensed catalogs establish those roles.

## Community, after content rights and identity

Public collections, creator favorites, richer showcases, and taste comparison extend the profile. Then add follows, reviews, comments and spoiler controls with reporting, moderation and deletion paths. A feed comes after curated discovery already works without one.

## Business capabilities, deliberately deferred

Creator submission UI and editorial moderation precede self-service publishing. MangaDex's API rules out advertising and paid services anywhere on this website or app while the integration is active; any future monetization requires retiring the MangaDex integration first. Subscriptions, entitlements and payments require idempotent signed webhooks and audited authorization. Creator payouts require accounting, identity, tax and fraud operations. Prefer providers for regulated functions. Defer panel-anchored discussion until artwork permissions, spoiler boundaries, moderation and takedown workflows exist. No marketplace, opaque recommendation system, native app, microservices, speculative plugin system, or extracted integration framework in this pilot.

An integration can become an open-source library only after two concrete application uses establish a stable seam. Keep fixes to upstream libraries small and independently reviewable.
