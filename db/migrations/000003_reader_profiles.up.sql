BEGIN;

-- A Work is the identity readers collect across editions. The existing series
-- rows are the currently hosted edition for each of the original fixtures.
CREATE TABLE works (
    id bigserial PRIMARY KEY,
    slug text NOT NULL UNIQUE CHECK (slug ~ '^[a-z0-9]+(-[a-z0-9]+)*$'),
    title text NOT NULL CHECK (length(trim(title)) BETWEEN 1 AND 160),
    created_at timestamptz NOT NULL DEFAULT now()
);

ALTER TABLE series ADD COLUMN work_id bigint;
INSERT INTO works (slug, title)
SELECT slug, title FROM series ORDER BY id
ON CONFLICT (slug) DO UPDATE SET title = EXCLUDED.title;
UPDATE series AS s SET work_id = w.id FROM works AS w WHERE w.slug = s.slug;
ALTER TABLE series ALTER COLUMN work_id SET NOT NULL;
ALTER TABLE series ADD CONSTRAINT series_work_id_fkey FOREIGN KEY (work_id) REFERENCES works(id) ON DELETE RESTRICT;
ALTER TABLE series ADD CONSTRAINT series_work_id_key UNIQUE (work_id);

CREATE TABLE reader_profiles (
    user_id bigint PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    handle text NOT NULL UNIQUE CHECK (handle ~ '^[a-z0-9][a-z0-9_-]{2,23}$'),
    display_name text NOT NULL CHECK (length(trim(display_name)) BETWEEN 1 AND 60),
    tagline text NOT NULL DEFAULT '' CHECK (length(tagline) <= 100),
    bio text NOT NULL DEFAULT '' CHECK (length(bio) <= 320),
    visibility text NOT NULL DEFAULT 'private' CHECK (visibility IN ('private', 'public')),
    theme text NOT NULL DEFAULT 'shelf' CHECK (theme IN ('shelf', 'night', 'coral')),
    show_reading_stats boolean NOT NULL DEFAULT false,
    showcase_order text NOT NULL DEFAULT 'favorites-first' CHECK (showcase_order IN ('favorites-first', 'stats-first')),
    showcase_version smallint NOT NULL DEFAULT 1 CHECK (showcase_version = 1),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX reader_profiles_public_handle_idx ON reader_profiles(handle) WHERE visibility = 'public';

CREATE TABLE profile_favorites (
    user_id bigint NOT NULL REFERENCES reader_profiles(user_id) ON DELETE CASCADE,
    work_id bigint NOT NULL REFERENCES works(id) ON DELETE RESTRICT,
    position smallint NOT NULL CHECK (position BETWEEN 1 AND 6),
    PRIMARY KEY (user_id, work_id),
    UNIQUE (user_id, position)
);
CREATE INDEX profile_favorites_work_idx ON profile_favorites(work_id, user_id);

COMMIT;
