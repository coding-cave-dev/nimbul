-- name: CreateCredential :one
INSERT INTO credentials (
  owner_id, provider, token_type, ciphertext, token_nonce, wrapped_dek, dek_nonce, expires_at 
) VALUES (
  $1, $2, $3, $4, $5, $6, $7, $8
)
RETURNING *;
