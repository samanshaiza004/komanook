BEGIN;

-- Catalog identity and its display metadata survive changes to a hosted edition.
ALTER TABLE works ADD COLUMN creator text NOT NULL DEFAULT '' CHECK (length(creator) <= 160);
ALTER TABLE works ADD COLUMN genre text NOT NULL DEFAULT '' CHECK (length(genre) <= 80);
ALTER TABLE works ADD COLUMN cover text NOT NULL DEFAULT '' CHECK (length(cover) <= 255);
UPDATE works w
SET creator = s.creator, genre = s.genre, cover = s.cover
FROM series s
WHERE s.work_id = w.id;

-- Library entries are private personal state. They do not create social activity.
CREATE TABLE reader_library (
    user_id bigint NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    work_id bigint NOT NULL REFERENCES works(id) ON DELETE RESTRICT,
    status text NOT NULL CHECK (status IN ('reading', 'plan_to_read', 'completed', 'on_hold', 'dropped')),
    rating smallint CHECK (rating IS NULL OR rating BETWEEN 1 AND 10),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, work_id)
);
CREATE INDEX reader_library_status_idx ON reader_library(user_id, status, updated_at DESC);

COMMIT;
