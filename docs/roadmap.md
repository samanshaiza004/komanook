# Komanook roadmap

## Foundation delivered in this slice

Independent Go/templ app, pinned build tooling, PostgreSQL schema/sqlc queries, explicit migrations, provisioned-account login/logout, SCS sessions, CSRF and role checks, public cached catalog/series pages, original CC0 sample comic, responsive independent reader, local/account progress, a bounded Datastar patch, MannerHTML tabs, private admin image quarantine with transactional River processing, CI and measured validation.

## Next: private content pilot

1. Operator workflow for rights evidence, draft chapter assembly, accessible transcripts, immutable image manifest and atomic publication. Store original source files privately; never publish an upload merely because processing succeeded.
2. Replace local quarantine/derivatives with private R2 buckets, signed upload intent, explicit byte/content limits, bounded processing container, cleanup/retention and separate delivery hostname. Configure CDN cache keys, rights revocation/purge and responsive image variants.
3. Account lifecycle: choose managed OIDC or fully vetted invitation/verification/recovery workflows. Add transactional email only with configured provider, delivery records, retries, suppression and abuse controls. Complete security review before public registration.
4. Database TLS, least-privilege runtime/release roles, backup/restore drill, deploy/rollback drill, secret rotation, operational alerts and ownership. Test worker failure/retry on the chosen deployment environment.
5. Test an actual licensed chapter at hundreds of pages on low-memory phones. Establish production budgets from that measurement, not these tiny fixtures. Accessibility review with screen reader users and native assistive technology.

## Public reading release

Personal library/follows/bookmarks and history retention/deletion, indexed catalog discovery and tags, pagination, structured SEO metadata/sitemap, content reporting and moderation, privacy/account deletion, terms and content-rights operations. Add PostgreSQL text search before introducing a separate search service. Revisit session duration and identity risk once real usage is known.

## Business capabilities, deliberately deferred

Creator submission UI and editorial moderation precede self-service publishing. Advertising requires consent/privacy and measured reader impact. Subscriptions, entitlements and payments require idempotent signed webhooks and audited authorization. Creator payouts require accounting, identity, tax and fraud operations. Prefer providers for regulated functions. No marketplace, social feed, recommendation engine, native app, microservices, speculative plugin system, or extracted integration framework in the foundation.

An integration can become an open-source library only after two concrete application uses establish a stable seam. Keep fixes to upstream libraries small and independently reviewable.
