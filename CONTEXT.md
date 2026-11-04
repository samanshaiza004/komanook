# Komanook domain language

Komanook connects readers through the manga they collect, discuss, and remember. These terms keep reader identity separate from account access and keep a manga's identity stable across editions.

## Reader identity

**Account**:
The private login identity used to authenticate and authorize actions.
_Avoid_: Profile

**Reader profile**:
An optional public or private expression of a reader's manga taste. It contains a handle, display name, personal note, chosen showcases, and only the aggregate reading details the reader enables.
_Avoid_: Account page

**Showcase**:
An ordered module a reader chooses to place on their profile, such as favorite manga or reading milestones.
_Avoid_: Feed

**Favorite**:
An ordered expression of personal admiration for a Work. It does not assert that the reader has started or completed it.
_Avoid_: Bookmark, rating

## Manga identity

**Work**:
The manga, manhwa, or webcomic readers recognize and collect across releases.
_Avoid_: Edition, hosted series

**Edition**:
A particular language, publisher, format, or materially revised release of a Work. Chapter order and reading position belong to an Edition.
_Avoid_: Work

**Hosted series**:
The currently hosted edition record used by Komanook's original pilot catalog. It is one edition of a Work, not the canonical identity for tracking across publishers.
_Avoid_: Work

**Reading milestone**:
A coarse fact derived from saved chapter positions, such as chapters with a saved place or a saved position at the final page. It is not a measure of reading time or proof that every page was read.
_Avoid_: Reading time, completion score
