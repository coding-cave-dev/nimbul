-- +goose Up
-- +goose StatementBegin
CREATE TABLE
    IF NOT EXISTS users (
        id char(26) PRIMARY KEY,
        email text NOT NULL UNIQUE,
        password_hash text NOT NULL,
        created_at timestamptz NOT NULL DEFAULT now (),
        updated_at timestamptz NOT NULL DEFAULT now ()
    );

-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS users;

-- +goose StatementEnd