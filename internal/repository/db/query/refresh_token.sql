-- name: UpsertRefreshToken :one
INSERT INTO refresh_tokens (
  uid,
  token,
  expires_at
) VALUES (
  sqlc.arg(uid),
  sqlc.arg(token),
  sqlc.arg(expires_at)
) ON CONFLICT(uid) DO UPDATE SET
  token = excluded.token,
  expires_at = excluded.expires_at
RETURNING
  token,
  created_at,
  expires_at;

-- name: GetRefreshToken :one
SELECT
  token,
  created_at,
  expires_at
FROM refresh_tokens
WHERE uid = sqlc.arg(uid)
LIMIT 1;

-- name: GetRefreshTokenByToken :one
SELECT
  uid,
  token,
  created_at,
  expires_at
FROM refresh_tokens
WHERE token = sqlc.arg(token)
LIMIT 1;

-- name: DeleteRefreshToken :execrows
DELETE FROM refresh_tokens
WHERE uid = sqlc.arg(uid);
