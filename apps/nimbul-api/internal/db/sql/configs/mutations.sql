-- name: CreateConfig :one
INSERT INTO repo_configs (
    id, owner_id, provider, repo_owner, repo_name, repo_full_name, 
    repo_clone_url, dockerfile_path, webhook_secret, webhook_id
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10
)
RETURNING *;

-- name: UpdateConfigWebhookID :one
UPDATE repo_configs
SET webhook_id = $2, updated_at = NOW()
WHERE id = $1
RETURNING *;

