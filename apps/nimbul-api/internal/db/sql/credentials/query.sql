-- name: GetCredentialByOwnerIDAndTokenType :one
SELECT * FROM credentials
WHERE owner_id = $1 AND token_type = $2 LIMIT 1;

-- name: GetUniqueProvidersByOwnerID :many
SELECT DISTINCT provider FROM credentials 
WHERE owner_id = $1 AND (expires_at IS NULL OR expires_at > NOW());