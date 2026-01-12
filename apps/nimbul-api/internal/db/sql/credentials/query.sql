-- name: GetCredentialByOwnerIDAndTokenType :one
SELECT * FROM credentials
WHERE owner_id = $1 AND token_type = $2 LIMIT 1;