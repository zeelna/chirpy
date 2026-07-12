-- name: CreateRefreshToken :one
INSERT INTO refresh_tokens (token, created_at, updated_at, expires_at, revoked_at, user_id)
VALUES (
           $1,
           NOW(),
           NOW(),
           NOW() + INTERVAL '60 DAYS',
           NULL,
           $2
       )
RETURNING *;

-- name: CheckRefreshToken :one
SELECT * FROM refresh_tokens WHERE token = $1
    AND refresh_tokens.revoked_at IS NULL
    AND refresh_tokens.expires_at > NOW();


-- name: GetUserFromRefreshToken :one
SELECT users.*
FROM users
 INNER JOIN refresh_tokens ON refresh_tokens.user_id = users.id
    WHERE refresh_tokens.token = $1
      AND refresh_tokens.revoked_at IS NULL
      AND refresh_tokens.expires_at > NOW();

-- name: RevokeRefreshToken :exec
UPDATE refresh_tokens
SET revoked_at = NOW(), updated_at = NOW()
WHERE token = $1;