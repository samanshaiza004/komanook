# Foundation measurements

Measured 2026-11-03 using Chromium 156.0.8078.4 on Apple M1 (darwin/arm64). Raw observations: [performance-results.json](performance-results.json). Reproduce with a running seeded application: `npm run bench`, or run `npm run test:local` for the complete isolated check.

## Cold navigation

Three fresh contexts per workflow; table reports medians, not percentiles or field data. Desktop: 1440×1000, unthrottled. Mobile: 390×844 CSS px, DPR 2, 4× CPU slowdown, 150 ms network latency, 200,000 B/s down, 75,000 B/s up. Navigation waits for network idle and another 300 ms for observation. LCP is the last observed candidate at that point, not an end-of-session field metric.

| Profile | Route | TTFB ms | LCP ms | Load ms | Transfer KiB | Requests | CLS |
| --- | --- | ---: | ---: | ---: | ---: | ---: | ---: |
| desktop | /catalog | 2 | 80 | 65 | 92.2 | 7 | 0.000 |
| desktop | /read/1 | 2 | 24 | 23 | 163.9 | 8 | 0.027 |
| throttled-mobile | /catalog | 4 | 948 | 949 | 92.2 | 7 | 0.000 |
| throttled-mobile | /read/1 | 2 | 692 | 668 | 129.2 | 7 | 0.000 |

The origin is local HTTP with a real local PostgreSQL database. There is no CDN, TLS or HTTP response compression; transfer sizes include response headers and only images actually requested in the initial viewport/lazy-loading margin. Production latency, compression, server hardware and cache hit rate will change these numbers. There is no benchmark against another platform and no performance-superiority claim.

## Browser assets

| Asset | Raw KiB | Gzip KiB | Brotli KiB |
| --- | ---: | ---: | ---: |
| account.js | 2.6 | 1.1 | 0.9 |
| app.css | 21.0 | 5.5 | 4.8 |
| app.js | 0.1 | 0.1 | 0.1 |
| reader.js | 8.1 | 2.7 | 2.3 |
| vendor/datastar-1.0.4.js | 32.8 | 13.1 | 11.9 |
| vendor/manner/logic.js | 0.3 | 0.2 | 0.1 |
| vendor/manner/shared.js | 2.9 | 1.0 | 0.8 |
| vendor/manner/tabs.js | 19.7 | 4.9 | 4.1 |

Compressed sizes are calculated artifacts, not observed local wire sizes. Catalog loads Datastar and CSS. Series additionally loads the MannerHTML tabs module and its two dependencies. Reader loads CSS and its own module, with no Datastar or MannerHTML runtime. Account pages load CSS and a small progress-import module. There are no remote font or analytics requests.

## Reader memory and images

Four original WebP fixture pages, each 1000×1400. After switching to pagination, sample JS heap after forced GC, select a page 40 times, force GC again, then sample.

| Profile | JS heap growth across three runs, bytes | Image sources retained in each run |
| --- | ---: | --- |
| desktop | 107764–108412 | 2, 2, 2 |
| throttled-mobile | 107868–109448 | 2, 2, 2 |

A source window of the current page plus one neighbor on each side is enforced in pagination. DOM nodes for all pages remain so the no-JS document and semantics stay simple. Continuous mode uses native lazy loading and keeps loaded sources. These heap samples do **not** measure native decoded-image/GPU memory, prove leak freedom, or validate a 200-page chapter on a low-memory phone. The 250-page stress fixture repeats one CC0 image, so its memory result does not represent 250 distinct full-resolution image decodes. It checks pagination and DOM behavior under a long document; a real licensed long chapter, decoded-memory profiling, adaptive resolutions and mobile assistive-technology checks remain release gates before a large public catalog. Per-image request durations and bytes are retained in the raw report.

The separate long-chapter exercise uses the current reader module, 250 pages, and 40 selections in paginated mode after forced GC.

| Profile | JS heap growth across three runs, bytes | DOM nodes per run | Image sources retained per run |
| --- | ---: | ---: | ---: |
| desktop | 17616–17728 | 5120, 5120, 5120 | 3, 3, 3 |
| throttled-mobile | 17792–17948 | 5120, 5120, 5120 | 3, 3, 3 |

Pagination retains at most three image sources around the current page (two at either end). DOM node counts are expected to remain proportional to chapter length because the entire no-JavaScript strip stays in the document.

## Budgets for this fixture

Aim for reader custom JS below 12 KiB raw, catalog initial transfer below 350 KiB, and zero unexpected layout shift. Browser tests assert source-window bounds, keyboard operation, persistence and narrow-screen overflow. These are regression targets for this slice, not production promises. Screenshots in [screenshots](screenshots/) were inspected at desktop and mobile widths. Automated axe checks supplement browser tests; they do not certify accessibility.
