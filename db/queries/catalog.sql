-- name: Catalog :many
SELECT * FROM series WHERE published = true
AND (sqlc.arg(search)::text = '' OR title ILIKE '%' || sqlc.arg(search)::text || '%' OR creator ILIKE '%' || sqlc.arg(search)::text || '%')
AND (sqlc.arg(genre)::text = '' OR genre = sqlc.arg(genre))
ORDER BY id LIMIT 24;

-- name: Series :one
SELECT * FROM series WHERE slug = $1 AND published = true;

-- name: Chapters :many
SELECT * FROM chapters WHERE series_id = $1 AND published = true ORDER BY number;

-- name: Chapter :one
SELECT c.*, s.title AS series_title, s.slug AS series_slug, s.rights
FROM chapters c JOIN series s ON s.id = c.series_id
WHERE c.id = $1 AND c.published = true AND s.published = true;

-- name: Pages :many
SELECT * FROM pages WHERE chapter_id = $1 ORDER BY number;
