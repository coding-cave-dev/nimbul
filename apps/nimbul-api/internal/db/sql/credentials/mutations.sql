-- name: CreateCredential :one
INSERT INTO credentials (
  owner_id, provider, token_type, ciphertext, token_nonce, wrapped_dek, dek_nonce, expires_at 
) VALUES (
  $1, $2, $3, $4, $5, $6, $7, $8
)
RETURNING *;

-- name: UpdateCredential :one
UPDATE credentials
SET 
  ciphertext = $4,
  token_nonce = $5,
  wrapped_dek = $6,
  dek_nonce = $7,
  expires_at = $8,
  last_used_at = NOW()
WHERE owner_id = $1 AND provider = $2 AND token_type = $3
RETURNING *;
