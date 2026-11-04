-- name: BlockMangaDexGroup :exec
INSERT INTO mangadex_group_removals (group_id, group_name, request_reference, created_by)
VALUES ($1, $2, $3, $4)
ON CONFLICT (group_id) DO NOTHING;

-- name: MangaDexBlockedGroupIDs :many
SELECT group_id FROM mangadex_group_removals ORDER BY created_at;

-- name: MangaDexGroupRemovals :many
SELECT group_id, group_name, request_reference, created_at FROM mangadex_group_removals ORDER BY created_at DESC;
