CREATE TABLE posts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    uid TEXT NOT NULL UNIQUE,
    author TEXT NOT NULL,
    text TEXT NOT NULL,
    images TEXT NOT NULL DEFAULT '[]',
    attachments TEXT NOT NULL DEFAULT '[]',
    comment_count INTEGER NOT NULL DEFAULT 0,
    collection_count INTEGER NOT NULL DEFAULT 0,
    like_count INTEGER NOT NULL DEFAULT 0,
    pinned INTEGER NOT NULL CHECK (pinned IN (0, 1)) DEFAULT 0,
    visibility TEXT NOT NULL CHECK (visibility IN ('PUBLIC', 'PRIVATE')) DEFAULT 'PUBLIC',
    latest_replied_on INTEGER NOT NULL DEFAULT (unixepoch()),
    ip TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL CHECK (status IN ('NORMAL', 'ARCHIVED')) DEFAULT 'NORMAL',
    created_at INTEGER NOT NULL DEFAULT (unixepoch()),
    updated_at INTEGER NOT NULL DEFAULT (unixepoch())
);

CREATE UNIQUE INDEX idx_post_uid ON posts (uid);
CREATE INDEX idx_author_uid ON posts (author);

CREATE TABLE tags (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE
);

CREATE TABLE post_tags (
    post_id INTEGER NOT NULL,
    tag_id INTEGER NOT NULL,
    PRIMARY KEY (post_id, tag_id)
);

CREATE INDEX idx_post_tags_post_id ON post_tags (post_id);
CREATE INDEX idx_post_tags_tag_id ON post_tags (tag_id);

CREATE TABLE post_likes (
    post_uid TEXT NOT NULL,
    user_uid TEXT NOT NULL,
    created_at INTEGER NOT NULL DEFAULT (unixepoch()),
    PRIMARY KEY (post_uid, user_uid)
);

CREATE INDEX idx_post_likes_user_uid ON post_likes (user_uid);
CREATE INDEX idx_post_likes_post_uid_created_at ON post_likes (post_uid, created_at);

CREATE TABLE post_collections (
    post_uid TEXT NOT NULL,
    user_uid TEXT NOT NULL,
    created_at INTEGER NOT NULL DEFAULT (unixepoch()),
    PRIMARY KEY (post_uid, user_uid)
);

CREATE INDEX idx_post_collections_user_uid ON post_collections (user_uid);
CREATE INDEX idx_post_collections_post_uid_created_at ON post_collections (post_uid, created_at);
