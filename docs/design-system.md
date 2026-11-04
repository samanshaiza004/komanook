# Komanook design system

## Direction: the reading shelf

For readers discovering independent manga and vertical comics, the catalog's main job is to get a compelling story into the reader. The signature is an oversized indigo shelf behind a triptych of original comic covers. Supporting UI is quiet and spacious. Stillness is the motion policy: respect reduced motion, use brief control-state transitions only, and never animate reading images.

| Token | Light | Dark | Role |
| --- | --- | --- | --- |
| Paper | `#f5f6fb` | `#121627` | page canvas |
| Surface | `#ffffff` | `#1b2136` | cards and controls |
| Ink | `#1c2444` | `#f0f2ff` | primary text |
| Muted | `#56617a` | `#afb8d1` | secondary text |
| Indigo | `#3730a3` | `#a7a9ff` | action and focus |
| Coral | `#fd927b` | `#fd927b` | illustration accent, never small white text |

Body: system sans (`system-ui`, platform UI fonts), avoiding a remote font request. Display uses the same family with strong weights and tight but legible tracking; comic dialogue uses local sans. A dedicated webfont is deferred until there is a licensed typeface that earns its transfer cost. Spacing: 4/8/12/16/24/32/48/64 px; 12px control radius, 20px card radius. Minimum control height 44px; focus 3px with offset; readable line length around 65ch. Covers use a stable 2:3 ratio and preserve the illustration. Reader pages preserve their intrinsic aspect ratio and never crop.

Dark mode follows the system using CSS. Reader mode changes do not change the document's text direction; RTL is page navigation semantics only. At 375px, navigation wraps cleanly, catalog covers use two columns and reader controls stack. Color is never the sole state indicator. Descriptive alternative text includes comic dialogue. Auth errors identify recovery actions without revealing account existence.

## Component approach

Tailwind CSS v4 compiles locally; the product's semantic CSS and custom tokens own the visual identity. shadcn-templ is a source distribution, not a runtime design mandate. Adopt a minimal code-owned button primitive after source review with upstream MIT attribution; remove its unnecessary class-merging and visual defaults. Other simple primitives are native labels, inputs, anchors, details and buttons. Complex shadcn controllers are deliberately excluded while MannerHTML supplies tabs. The research document compares the component alternatives and licensing.

## DOM ownership contract

| Owner | Allowed subtree and state | Rules |
| --- | --- | --- |
| templ/server | Initial document, semantic content, stable IDs | escaped text, stable keys; full navigation remains usable |
| Datastar | `#catalog-results` only | finite same-origin GET; patch results, never search controls, tabs, reader, forms or focused elements |
| MannerHTML | `manner-tabs` on series pages | exclusively owns roles, selection, hidden, tabindex, keyboard focus inside it; all panels visible without JS |
| shadcn-derived primitive | button HTML and classes | no scripts, no global listeners or mutation controller |
| reader module | `[data-reader]` | exclusive image visibility/sources, mode and progress; never patched by Datastar |
| account module | `[data-progress-import]` | reads only the guest progress namespace; sends a bounded import only after an explicit button action |
| browser | native forms, details, links | preserve default validation and keyboard behavior |

Load MannerHTML's tabs entry only where needed; the custom element handles connection/disconnection. Do not register the same tab with a shadcn or Datastar controller. There is no automatic MannerHTML structural mutation reconciliation; future changes must call its `refresh()` deliberately or replace the entire custom element and restore focus. Current patches avoid this entirely. Datastar search keeps focus in its persistent form; a polite live region inside results announces the result count. Browser tests assert focus and MannerHTML keyboard behavior after a Datastar patch on the series page's related-story area.

Datastar's default evaluator requires CSP `unsafe-eval`; scripts otherwise come only from local static assets, with no inline scripts, remote CDNs, raw user HTML, or unsafe-inline script permission. Consider the upstream CSP build if adoption broadens. The reader does not require this exception and remains replaceable.
