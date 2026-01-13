-- name: GetConfigByID :one
SELECT * FROM repo_configs
WHERE id = $1 LIMIT 1;

-- name: GetConfigsByOwnerID :many
SELECT * FROM repo_configs
WHERE owner_id = $1
ORDER BY created_at DESC;

-- name: GetConfigByOwnerIDAndRepoFullName :one
SELECT * FROM repo_configs
WHERE owner_id = $1 AND repo_full_name = $2 LIMIT 1;

-- name: GetConfigByWebhookID :one
SELECT * FROM repo_configs
WHERE webhook_id = $1 LIMIT 1;

