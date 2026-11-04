-- name: GetProgress :one
SELECT page FROM reading_progress WHERE user_id = $1 AND chapter_id = $2;

-- name: SaveProgress :exec
INSERT INTO reading_progress (user_id, chapter_id, page) VALUES ($1,$2,$3)
ON CONFLICT(user_id,chapter_id) DO UPDATE SET page = EXCLUDED.page, updated_at = now();

-- name: ImportProgress :execrows
INSERT INTO reading_progress (user_id, chapter_id, page) VALUES ($1,$2,$3)
ON CONFLICT(user_id,chapter_id) DO NOTHING;
