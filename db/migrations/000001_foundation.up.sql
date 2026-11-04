BEGIN;
CREATE TABLE users (
    id bigserial PRIMARY KEY,
    email text NOT NULL UNIQUE CHECK (email = lower(email)),
    password_hash bytea NOT NULL,
    role text NOT NULL DEFAULT 'reader' CHECK (role IN ('reader','admin')),
    disabled boolean NOT NULL DEFAULT false,
    auth_version integer NOT NULL DEFAULT 1,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE sessions (token text PRIMARY KEY, data bytea NOT NULL, expiry timestamptz NOT NULL);
CREATE INDEX sessions_expiry_idx ON sessions(expiry);
CREATE TABLE login_limits (key text PRIMARY KEY, attempts integer NOT NULL, expires_at timestamptz NOT NULL);
CREATE INDEX login_limits_expiry_idx ON login_limits(expires_at);
CREATE TABLE series (
    id bigserial PRIMARY KEY,
    slug text NOT NULL UNIQUE,
    title text NOT NULL,
    creator text NOT NULL,
    synopsis text NOT NULL,
    genre text NOT NULL,
    format text NOT NULL,
    cover text NOT NULL,
    rights text NOT NULL CHECK (length(rights) > 0),
    published boolean NOT NULL DEFAULT false
);
CREATE TABLE chapters (
    id bigserial PRIMARY KEY,
    series_id bigint NOT NULL REFERENCES series(id),
    number integer NOT NULL CHECK (number > 0),
    title text NOT NULL,
    published boolean NOT NULL DEFAULT false,
    UNIQUE(series_id, number)
);
CREATE TABLE pages (
    chapter_id bigint NOT NULL REFERENCES chapters(id) ON DELETE CASCADE,
    number integer NOT NULL CHECK (number > 0),
    image text NOT NULL,
    width integer NOT NULL CHECK (width > 0),
    height integer NOT NULL CHECK (height > 0),
    transcript text NOT NULL,
    PRIMARY KEY(chapter_id, number)
);
CREATE TABLE reading_progress (
    user_id bigint NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    chapter_id bigint NOT NULL REFERENCES chapters(id) ON DELETE CASCADE,
    page integer NOT NULL CHECK (page > 0),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY(user_id, chapter_id)
);
CREATE TABLE uploads (
    id text PRIMARY KEY,
    user_id bigint NOT NULL REFERENCES users(id),
    state text NOT NULL DEFAULT 'pending' CHECK (state IN ('pending','ready')),
    output text NOT NULL DEFAULT '',
    width integer NOT NULL DEFAULT 0,
    height integer NOT NULL DEFAULT 0,
    created_at timestamptz NOT NULL DEFAULT now()
);
COMMIT;
