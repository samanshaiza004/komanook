# Komanook roadmap

## Foundation delivered in this slice

Independent Go/templ app, pinned build tooling, PostgreSQL schema/sqlc queries, explicit migrations, provisioned-account login/logout, SCS sessions, CSRF and role checks, public cached catalog/series pages, original CC0 sample comics, responsive independent reader, local/account progress, a bounded Datastar patch, MannerHTML tabs, private admin image quarantine with transactional River processing, a MangaDex API read-through with scanlation credits and recorded group removals, CI and measured validation.

## 0.1.1: foundation acceptance

Before calling the foundation accepted, make Tailwind output deterministic with explicit template sources, partition guest and account reading progress, provide an explicit non-overwriting import, stress the reader against a 250-page chapter fixture, and get every job in GitHub Actions green on the committed source. A local pass is useful evidence but does not replace that remote acceptance check.

## Next: private content pilot

1. Operator workflow for rights evidence, draft chapter assembly, accessible transcripts, immutable image manifest and atomic publication. Store original source files privately; never publish an upload merely because processing succeeded.
2. Replace local quarantine/derivatives with private R2 buckets, signed upload intent, explicit byte/content limits, bounded processing container, cleanup/retention and separate delivery hostname. Configure CDN cache keys, rights revocation/purge and responsive image variants.
3. Account lifecycle: choose managed OIDC or fully vetted invitation/verification/recovery workflows. Add transactional email only with configured provider, delivery records, retries, suppression and abuse controls. Complete security review before public registration.
4. Database TLS, least-privilege runtime/release roles, backup/restore drill, deploy/rollback drill, secret rotation, operational alerts and ownership. Test worker failure/retry on the chosen deployment environment.
5. Test an actual licensed chapter at hundreds of pages on low-memory phones. Establish production budgets from that measurement, not these tiny fixtures. Accessibility review with screen reader users and native assistive technology.

## Public reading release

Personal library/follows/bookmarks and history retention/deletion, indexed catalog discovery and tags, pagination, structured SEO metadata/sitemap, content reporting and moderation, privacy/account deletion, terms and content-rights operations. Add PostgreSQL text search before introducing a separate search service. Revisit session duration and identity risk once real usage is known.

Before building universal tracking or customizable profiles, define Works, Editions and hosted chapters, plus creator identities and ordered work-credit relationships. Personal libraries should follow a canonical Work and retain a chosen Edition; chapter progress stays edition-specific. Keep the current `series` fixture mapping as a pilot detail until licensed content and real tracking use cases validate the migration.

## Community, after content rights and identity

Profiles, public and private collections, creator favorites and showcases come after the Work/Edition model and account privacy choices. Then add follows, reviews, comments and spoiler controls with reporting, moderation and deletion paths. This prepares for a manga community without introducing social feeds or profile customization into the foundation slice.

## Business capabilities, deliberately deferred

Creator submission UI and editorial moderation precede self-service publishing. MangaDex's API rules out advertising and paid services anywhere on this website or app while the integration is active; any future monetization requires retiring the MangaDex integration first. Subscriptions, entitlements and payments require idempotent signed webhooks and audited authorization. Creator payouts require accounting, identity, tax and fraud operations. Prefer providers for regulated functions. No marketplace, social feed, recommendation engine, native app, microservices, speculative plugin system, or extracted integration framework in the foundation.

An integration can become an open-source library only after two concrete application uses establish a stable seam. Keep fixes to upstream libraries small and independently reviewable.
