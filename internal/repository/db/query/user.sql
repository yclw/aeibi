-- name: CreateUser :one
INSERT INTO users (
  uid,
  username,
  email,
  nickname,
  password_hash,
  avatar_url
) VALUES (
  sqlc.arg(uid),
  sqlc.arg(username),
  sqlc.arg(email),
  sqlc.arg(nickname),
  sqlc.arg(password_hash),
  sqlc.arg(avatar_url)
) RETURNING
  uid,
  username,
  role,
  email,
  nickname,
  avatar_url;

-- name: GetUserByUid :one
SELECT
  uid,
  username,
  role,
  email,
  nickname,
  avatar_url
FROM users
WHERE uid = sqlc.arg(uid)
  AND status = 'NORMAL'
LIMIT 1;

-- name: GetUserAuthByAccount :one
SELECT
  uid,
  username,
  role,
  email,
  nickname,
  avatar_url,
  password_hash
FROM users
WHERE (username = sqlc.arg(account) OR email = sqlc.arg(account))
  AND status = 'NORMAL'
LIMIT 1;

-- name: ListUsers :many
SELECT
  uid,
  username,
  role,
  email,
  nickname,
  avatar_url
FROM users
WHERE status = 'NORMAL'
  AND (
    LENGTH(CAST(sqlc.arg(filter) AS TEXT)) = 0
    OR username LIKE '%' || CAST(sqlc.arg(filter) AS TEXT) || '%'
    OR email LIKE '%' || CAST(sqlc.arg(filter) AS TEXT) || '%'
    OR nickname LIKE '%' || CAST(sqlc.arg(filter) AS TEXT) || '%'
  )
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(limit)
OFFSET sqlc.arg(offset);

-- name: CountUsers :one
SELECT COUNT(1)
FROM users
WHERE status = 'NORMAL'
  AND (
    LENGTH(CAST(sqlc.arg(filter) AS TEXT)) = 0
    OR username LIKE '%' || CAST(sqlc.arg(filter) AS TEXT) || '%'
    OR email LIKE '%' || CAST(sqlc.arg(filter) AS TEXT) || '%'
    OR nickname LIKE '%' || CAST(sqlc.arg(filter) AS TEXT) || '%'
  );

-- name: UpdateUserByUid :one
UPDATE users
SET
  username   = COALESCE(sqlc.narg(username), username),
  email      = COALESCE(sqlc.narg(email), email),
  nickname   = COALESCE(sqlc.narg(nickname), nickname),
  avatar_url = COALESCE(sqlc.narg(avatar_url), avatar_url),
  updated_at = unixepoch()
WHERE uid = sqlc.arg(uid)
  AND status = 'NORMAL'
RETURNING uid, username, role, email, nickname, avatar_url;

-- name: ArchiveUserByUid :execrows
UPDATE users
SET
  status = 'ARCHIVED',
  updated_at = unixepoch()
WHERE uid = sqlc.arg(uid)
  AND status = 'NORMAL';

-- name: GetUserPasswordHashByUid :one
SELECT password_hash
FROM users
WHERE uid = sqlc.arg(uid)
  AND status = 'NORMAL'
LIMIT 1;

-- name: UpdateUserPasswordHashByUid :execrows
UPDATE users
SET
  password_hash = sqlc.arg(password_hash),
  updated_at = unixepoch()
WHERE uid = sqlc.arg(uid)
  AND status = 'NORMAL';
