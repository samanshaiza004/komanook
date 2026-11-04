BEGIN;
CREATE TABLE mangadex_group_removals (
    group_id text PRIMARY KEY CHECK (group_id ~* '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'),
    group_name text NOT NULL CHECK (length(trim(group_name)) BETWEEN 1 AND 120),
    request_reference text NOT NULL CHECK (length(trim(request_reference)) BETWEEN 1 AND 500),
    created_by bigint NOT NULL REFERENCES users(id),
    created_at timestamptz NOT NULL DEFAULT now()
);
COMMIT;
