-- name: CreatePost :one
INSERT INTO posts (
  uid,
  author,
  text,
  images,
  attachments,
  visibility,
  pinned,
  ip
) VALUES (
  sqlc.arg(uid),
  sqlc.arg(author),
  sqlc.arg(text),
  sqlc.arg(images),
  sqlc.arg(attachments),
  sqlc.arg(visibility),
  sqlc.arg(pinned),
  sqlc.arg(ip)
) RETURNING
  id,
  uid,
  author,
  text,
  images,
  attachments,
  comment_count,
  collection_count,
  like_count,
  pinned,
  visibility,
  latest_replied_on,
  ip,
  status,
  created_at,
  updated_at;

-- name: UpsertTag :one
INSERT INTO tags (name) VALUES (sqlc.arg(name))
ON CONFLICT(name) DO UPDATE SET
  name = excluded.name
RETURNING id, name;

-- name: AddPostTag :exec
INSERT INTO post_tags (
  post_id,
  tag_id
) VALUES (
  sqlc.arg(post_id),
  sqlc.arg(tag_id)
) ON CONFLICT DO NOTHING;

-- name: CountPublicPosts :one
SELECT COUNT(1)
FROM posts p
WHERE p.status = 'NORMAL'
  AND p.visibility = 'PUBLIC'
  AND (
    LENGTH(CAST(sqlc.arg(author) AS TEXT)) = 0
    OR p.author = sqlc.arg(author)
  )
  AND (
    LENGTH(CAST(sqlc.arg(visibility) AS TEXT)) = 0
    OR p.visibility = sqlc.arg(visibility)
  )
  AND (
    LENGTH(CAST(sqlc.arg(search) AS TEXT)) = 0
    OR p.text LIKE '%' || sqlc.arg(search) || '%'
  )
  AND (
    LENGTH(CAST(sqlc.arg(tag) AS TEXT)) = 0
    OR EXISTS (
      SELECT 1
      FROM post_tags pt
      JOIN tags t ON t.id = pt.tag_id
      WHERE pt.post_id = p.id
        AND t.name = sqlc.arg(tag)
    )
  );

-- name: ListPublicPosts :many
SELECT
  p.id,
  p.uid,
  p.author,
  p.text,
  p.images,
  p.attachments,
  p.comment_count,
  p.collection_count,
  p.like_count,
  p.pinned,
  p.visibility,
  p.latest_replied_on,
  p.ip,
  p.status,
  p.created_at,
  p.updated_at,
  COALESCE(u.nickname, '') AS author_nickname,
  COALESCE(u.avatar_url, '') AS author_avatar_url
FROM posts p
LEFT JOIN users u ON u.uid = p.author
WHERE p.status = 'NORMAL'
  AND p.visibility = 'PUBLIC'
  AND (
    LENGTH(CAST(sqlc.arg(author) AS TEXT)) = 0
    OR p.author = sqlc.arg(author)
  )
  AND (
    LENGTH(CAST(sqlc.arg(visibility) AS TEXT)) = 0
    OR p.visibility = sqlc.arg(visibility)
  )
  AND (
    LENGTH(CAST(sqlc.arg(search) AS TEXT)) = 0
    OR p.text LIKE '%' || sqlc.arg(search) || '%'
  )
  AND (
    LENGTH(CAST(sqlc.arg(tag) AS TEXT)) = 0
    OR EXISTS (
      SELECT 1
      FROM post_tags pt
      JOIN tags t ON t.id = pt.tag_id
      WHERE pt.post_id = p.id
        AND t.name = sqlc.arg(tag)
    )
  )
ORDER BY p.created_at DESC, p.id DESC
LIMIT sqlc.arg(limit)
OFFSET sqlc.arg(offset);

-- name: CountMyPosts :one
SELECT COUNT(1)
FROM posts p
WHERE p.status = 'NORMAL'
  AND p.author = sqlc.arg(author)
  AND (
    LENGTH(CAST(sqlc.arg(visibility) AS TEXT)) = 0
    OR p.visibility = sqlc.arg(visibility)
  )
  AND (
    LENGTH(CAST(sqlc.arg(search) AS TEXT)) = 0
    OR p.text LIKE '%' || sqlc.arg(search) || '%'
  )
  AND (
    LENGTH(CAST(sqlc.arg(tag) AS TEXT)) = 0
    OR EXISTS (
      SELECT 1
      FROM post_tags pt
      JOIN tags t ON t.id = pt.tag_id
      WHERE pt.post_id = p.id
        AND t.name = sqlc.arg(tag)
    )
  );

-- name: ListMyPosts :many
SELECT
  p.id,
  p.uid,
  p.author,
  p.text,
  p.images,
  p.attachments,
  p.comment_count,
  p.collection_count,
  p.like_count,
  p.pinned,
  p.visibility,
  p.latest_replied_on,
  p.ip,
  p.status,
  p.created_at,
  p.updated_at,
  COALESCE(u.nickname, '') AS author_nickname,
  COALESCE(u.avatar_url, '') AS author_avatar_url
FROM posts p
LEFT JOIN users u ON u.uid = p.author
WHERE p.status = 'NORMAL'
  AND p.author = sqlc.arg(author)
  AND (
    LENGTH(CAST(sqlc.arg(visibility) AS TEXT)) = 0
    OR p.visibility = sqlc.arg(visibility)
  )
  AND (
    LENGTH(CAST(sqlc.arg(search) AS TEXT)) = 0
    OR p.text LIKE '%' || sqlc.arg(search) || '%'
  )
  AND (
    LENGTH(CAST(sqlc.arg(tag) AS TEXT)) = 0
    OR EXISTS (
      SELECT 1
      FROM post_tags pt
      JOIN tags t ON t.id = pt.tag_id
      WHERE pt.post_id = p.id
        AND t.name = sqlc.arg(tag)
    )
  )
ORDER BY p.created_at DESC, p.id DESC
LIMIT sqlc.arg(limit)
OFFSET sqlc.arg(offset);

-- name: GetPostByUid :one
SELECT
  p.id,
  p.uid,
  p.author,
  p.text,
  p.images,
  p.attachments,
  p.comment_count,
  p.collection_count,
  p.like_count,
  p.pinned,
  p.visibility,
  p.latest_replied_on,
  p.ip,
  p.status,
  p.created_at,
  p.updated_at,
  COALESCE(u.nickname, '') AS author_nickname,
  COALESCE(u.avatar_url, '') AS author_avatar_url
FROM posts p
LEFT JOIN users u ON u.uid = p.author
WHERE p.uid = sqlc.arg(uid)
  AND p.status = 'NORMAL'
LIMIT 1;

-- name: GetPublicPostByUid :one
SELECT
  p.id,
  p.uid,
  p.author,
  p.text,
  p.images,
  p.attachments,
  p.comment_count,
  p.collection_count,
  p.like_count,
  p.pinned,
  p.visibility,
  p.latest_replied_on,
  p.ip,
  p.status,
  p.created_at,
  p.updated_at,
  COALESCE(u.nickname, '') AS author_nickname,
  COALESCE(u.avatar_url, '') AS author_avatar_url
FROM posts p
LEFT JOIN users u ON u.uid = p.author
WHERE p.uid = sqlc.arg(uid)
  AND p.visibility = 'PUBLIC'
  AND p.status = 'NORMAL'
LIMIT 1;

-- name: GetPostByUidAndAuthor :one
SELECT
  p.id,
  p.uid,
  p.author,
  p.text,
  p.images,
  p.attachments,
  p.comment_count,
  p.collection_count,
  p.like_count,
  p.pinned,
  p.visibility,
  p.latest_replied_on,
  p.ip,
  p.status,
  p.created_at,
  p.updated_at,
  COALESCE(u.nickname, '') AS author_nickname,
  COALESCE(u.avatar_url, '') AS author_avatar_url
FROM posts p
LEFT JOIN users u ON u.uid = p.author
WHERE p.uid = sqlc.arg(uid)
  AND p.author = sqlc.arg(author)
  AND p.status = 'NORMAL'
LIMIT 1;

-- name: ListPostTagsByUid :many
SELECT t.name
FROM tags t
JOIN post_tags pt ON pt.tag_id = t.id
JOIN posts p ON p.id = pt.post_id
WHERE p.uid = sqlc.arg(uid)
ORDER BY t.id;

-- name: CountMyCollections :one
SELECT COUNT(1)
FROM post_collections pc
JOIN posts p ON p.uid = pc.post_uid
WHERE pc.user_uid = sqlc.arg(user_uid)
  AND p.status = 'NORMAL'
  AND (p.visibility = 'PUBLIC' OR p.author = sqlc.arg(user_uid))
  AND (
    LENGTH(CAST(sqlc.arg(author) AS TEXT)) = 0
    OR p.author = sqlc.arg(author)
  )
  AND (
    LENGTH(CAST(sqlc.arg(visibility) AS TEXT)) = 0
    OR p.visibility = sqlc.arg(visibility)
  )
  AND (
    LENGTH(CAST(sqlc.arg(search) AS TEXT)) = 0
    OR p.text LIKE '%' || sqlc.arg(search) || '%'
  )
  AND (
    LENGTH(CAST(sqlc.arg(tag) AS TEXT)) = 0
    OR EXISTS (
      SELECT 1
      FROM post_tags pt
      JOIN tags t ON t.id = pt.tag_id
      WHERE pt.post_id = p.id
        AND t.name = sqlc.arg(tag)
    )
  );

-- name: ListMyCollections :many
SELECT
  p.id,
  p.uid,
  p.author,
  p.text,
  p.images,
  p.attachments,
  p.comment_count,
  p.collection_count,
  p.like_count,
  p.pinned,
  p.visibility,
  p.latest_replied_on,
  p.ip,
  p.status,
  p.created_at,
  p.updated_at,
  COALESCE(u.nickname, '') AS author_nickname,
  COALESCE(u.avatar_url, '') AS author_avatar_url
FROM post_collections pc
JOIN posts p ON p.uid = pc.post_uid
LEFT JOIN users u ON u.uid = p.author
WHERE pc.user_uid = sqlc.arg(user_uid)
  AND p.status = 'NORMAL'
  AND (p.visibility = 'PUBLIC' OR p.author = sqlc.arg(user_uid))
  AND (
    LENGTH(CAST(sqlc.arg(author) AS TEXT)) = 0
    OR p.author = sqlc.arg(author)
  )
  AND (
    LENGTH(CAST(sqlc.arg(visibility) AS TEXT)) = 0
    OR p.visibility = sqlc.arg(visibility)
  )
  AND (
    LENGTH(CAST(sqlc.arg(search) AS TEXT)) = 0
    OR p.text LIKE '%' || sqlc.arg(search) || '%'
  )
  AND (
    LENGTH(CAST(sqlc.arg(tag) AS TEXT)) = 0
    OR EXISTS (
      SELECT 1
      FROM post_tags pt
      JOIN tags t ON t.id = pt.tag_id
      WHERE pt.post_id = p.id
        AND t.name = sqlc.arg(tag)
    )
  )
ORDER BY pc.created_at DESC, p.id DESC
LIMIT sqlc.arg(limit)
OFFSET sqlc.arg(offset);

-- name: ArchivePostByUidAndAuthor :execrows
UPDATE posts
SET
  status = 'ARCHIVED',
  updated_at = unixepoch()
WHERE uid = sqlc.arg(uid)
  AND author = sqlc.arg(author)
  AND status = 'NORMAL';

-- name: InsertPostLike :execrows
INSERT INTO post_likes (
  post_uid,
  user_uid
) VALUES (
  sqlc.arg(post_uid),
  sqlc.arg(user_uid)
) ON CONFLICT DO NOTHING;

-- name: DeletePostLike :execrows
DELETE FROM post_likes
WHERE post_uid = sqlc.arg(post_uid)
  AND user_uid = sqlc.arg(user_uid);

-- name: UpdatePostLikeCount :execrows
UPDATE posts
SET
  like_count = CASE
    WHEN like_count + sqlc.arg(delta) < 0 THEN 0
    ELSE like_count + sqlc.arg(delta)
  END,
  updated_at = unixepoch()
WHERE uid = sqlc.arg(uid)
  AND status = 'NORMAL';

-- name: InsertPostCollection :execrows
INSERT INTO post_collections (
  post_uid,
  user_uid
) VALUES (
  sqlc.arg(post_uid),
  sqlc.arg(user_uid)
) ON CONFLICT DO NOTHING;

-- name: DeletePostCollection :execrows
DELETE FROM post_collections
WHERE post_uid = sqlc.arg(post_uid)
  AND user_uid = sqlc.arg(user_uid);

-- name: UpdatePostCollectionCount :execrows
UPDATE posts
SET
  collection_count = CASE
    WHEN collection_count + sqlc.arg(delta) < 0 THEN 0
    ELSE collection_count + sqlc.arg(delta)
  END,
  updated_at = unixepoch()
WHERE uid = sqlc.arg(uid)
  AND status = 'NORMAL';

-- name: UpdatePostByUidAndAuthor :one
UPDATE posts
SET
  text = COALESCE(sqlc.narg(text), text),
  images = COALESCE(sqlc.narg(images), images),
  attachments = COALESCE(sqlc.narg(attachments), attachments),
  visibility = COALESCE(sqlc.narg(visibility), visibility),
  pinned = CASE
    WHEN sqlc.narg(pinned) IS NULL THEN pinned
    ELSE CASE WHEN CAST(sqlc.narg(pinned) AS INTEGER) = 1 THEN 1 ELSE 0 END
  END,
  updated_at = unixepoch()
WHERE uid = sqlc.arg(uid)
  AND author = sqlc.arg(author)
  AND status = 'NORMAL'
RETURNING
  id,
  uid,
  author,
  text,
  images,
  attachments,
  comment_count,
  collection_count,
  like_count,
  pinned,
  visibility,
  latest_replied_on,
  ip,
  status,
  created_at,
  updated_at;

-- name: DeletePostTags :exec
DELETE FROM post_tags
WHERE post_id = sqlc.arg(post_id);
