-- name: UserByEmail :one
SELECT * FROM users WHERE email = $1;

-- name: UserByID :one
SELECT * FROM users WHERE id = $1;

-- name: ProvisionUser :one
INSERT INTO users (email, password_hash, role) VALUES ($1, $2, $3)
ON CONFLICT (email) DO UPDATE SET password_hash = EXCLUDED.password_hash, role = EXCLUDED.role, auth_version = users.auth_version + 1
RETURNING *;

-- name: LoginAttempt :one
INSERT INTO login_limits (key, attempts, expires_at) VALUES ($1, 1, now() + interval '15 minutes')
ON CONFLICT (key) DO UPDATE SET
attempts = CASE WHEN login_limits.expires_at < now() THEN 1 ELSE login_limits.attempts + 1 END,
expires_at = CASE WHEN login_limits.expires_at < now() THEN now() + interval '15 minutes' ELSE login_limits.expires_at END
RETURNING attempts;

-- name: ProfileByUserID :one
SELECT * FROM reader_profiles WHERE user_id = $1;

-- name: PublicProfileByHandle :one
SELECT p.*
FROM reader_profiles p
JOIN users u ON u.id = p.user_id AND u.disabled = false
WHERE p.handle = $1 AND p.visibility = 'public';

-- name: UpsertReaderProfile :one
INSERT INTO reader_profiles (
    user_id, handle, display_name, tagline, bio, visibility, theme,
    show_reading_stats, showcase_order, showcase_version
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,1)
ON CONFLICT (user_id) DO UPDATE SET
    handle = EXCLUDED.handle,
    display_name = EXCLUDED.display_name,
    tagline = EXCLUDED.tagline,
    bio = EXCLUDED.bio,
    visibility = EXCLUDED.visibility,
    theme = EXCLUDED.theme,
    show_reading_stats = EXCLUDED.show_reading_stats,
    showcase_order = EXCLUDED.showcase_order,
    showcase_version = 1,
    updated_at = now()
RETURNING *;

-- name: DeleteProfileFavorites :exec
DELETE FROM profile_favorites WHERE user_id = $1;

-- name: AddProfileFavorite :exec
INSERT INTO profile_favorites (user_id, work_id, position) VALUES ($1,$2,$3);

-- name: ProfileFavoritesByUserID :many
SELECT w.id AS work_id, w.slug AS work_slug, w.title AS work_title,
       s.creator, s.cover, s.genre, f.position
FROM profile_favorites f
JOIN works w ON w.id = f.work_id
JOIN series s ON s.work_id = w.id AND s.published = true
WHERE f.user_id = $1
ORDER BY f.position;

-- name: PublishedWorks :many
SELECT w.id AS work_id, w.slug AS work_slug, w.title AS work_title,
       s.creator, s.cover, s.genre
FROM works w
JOIN series s ON s.work_id = w.id AND s.published = true
ORDER BY w.title, w.id
LIMIT 100;

-- name: PublishedWorkByID :one
SELECT w.id AS work_id, w.slug AS work_slug, w.title AS work_title,
       s.creator, s.cover, s.genre
FROM works w
JOIN series s ON s.work_id = w.id AND s.published = true
WHERE w.id = $1;

-- name: ProfileReadingStats :one
WITH chapter_pages AS (
    SELECT c.id, count(p.number)::int AS page_count
    FROM chapters c
    JOIN series s ON s.id = c.series_id AND s.published = true
    JOIN pages p ON p.chapter_id = c.id
    WHERE c.published = true
    GROUP BY c.id
)
SELECT count(*)::bigint AS saved_chapters,
       count(*) FILTER (WHERE rp.page >= cp.page_count)::bigint AS final_pages_reached
FROM reading_progress rp
JOIN chapter_pages cp ON cp.id = rp.chapter_id
WHERE rp.user_id = $1;
