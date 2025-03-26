-- name: CreateUser :one
INSERT INTO users (id, created_at, updated_at, name)
VALUES (
    $1,
    $2,
    $3,
    $4
)
RETURNING *;

-- name: GetUser :one
SELECT * FROM users
WHERE name = $1
LIMIT 1;

-- name: GetFeedFromURL :one
SELECT * FROM feeds
WHERE url = $1
LIMIT 1;

-- name: DeleteAllUsers :exec
DELETE FROM users;

-- name: GetUsers :many
SELECT * FROM users;

-- name: GetFeeds :many
SELECT * from feeds;

-- name: CreateFeed :one
INSERT INTO feeds (id, created_at, updated_at, name, url, user_id)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetPostsForUsers :many
SELECT p.* FROM posts p
JOIN feeds f on p.feed_id = f.id
JOIN feed_follows ff ON f.id = ff.feed_id
WHERE ff.user_id = $1
ORDER BY p.published_at DESC
LIMIT $2;

-- name: GetFeedsWithUsers :many
SELECT f.id, f.created_at, f.updated_at, f.name, f.url, f.user_id, u.name as user_name
FROM feeds f
JOIN users u ON f.user_id = u.id;

-- name: GetFeedFollowsWithUser :many
SELECT ff.id, ff.created_at, ff.updated_at, ff.user_id, ff.feed_id, f.name as feedname, u.name as username
FROM feed_follows ff
JOIN users u ON ff.user_id = u.id
JOIN feeds f on ff.feed_id = f.id
WHERE u.name = $1;

-- name: CreatePost :one
INSERT INTO posts (id, title, url, description, published_at, feed_id)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (url) DO NOTHING
RETURNING *;

-- name: CreateFeedFollow :one
WITH inserted_feed_follows AS (
INSERT INTO feed_follows (id, created_at, updated_at, user_id, feed_id)
VALUES ($1, $2, $3, $4, $5)
RETURNING *
)
SELECT inserted_feed_follows.*, feeds.name as feed_name, users.name as user_name
FROM inserted_feed_follows
JOIN users ON inserted_feed_follows.user_id = users.id
JOIN feeds ON inserted_feed_follows.feed_id = feeds.id;

-- name: Unfollow :exec
DELETE FROM feed_follows WHERE user_id = $1 AND feed_id = $2;

-- name: MarkFeedFetched :exec
UPDATE feeds
SET updated_at = NOW(), last_fetched_at = NOW()
WHERE id = $1;

-- name: GetNextFeedToFetch :one
SELECT * FROM feeds
ORDER BY last_fetched_at NULLS FIRST;