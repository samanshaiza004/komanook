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
