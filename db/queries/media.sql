-- name: CreateUpload :exec
INSERT INTO uploads(id, user_id) VALUES ($1,$2);

-- name: Upload :one
SELECT * FROM uploads WHERE id = $1;

-- name: FinishUpload :exec
UPDATE uploads SET state = 'ready', output = $2, width = $3, height = $4 WHERE id = $1;
