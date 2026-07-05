-- +goose up
CREATE TABLE refresh_tokens(
   token TEXT PRIMARY KEY,
   created_at TIMESTAMP NOT NULL,
   updated_at TIMESTAMP NOT NULL,
   expires_at TIMESTAMP NOT NULL,
   revoked_at TIMESTAMP,

   user_id UUID NOT NULL,
   CONSTRAINT no_orphaned_tokens
       FOREIGN KEY (user_id)
           REFERENCES users(id)
           ON DELETE CASCADE
);

-- +goose down
DROP TABLE refresh_tokens;