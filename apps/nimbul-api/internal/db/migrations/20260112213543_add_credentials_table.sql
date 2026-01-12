-- +goose Up
-- +goose StatementBegin
create table
    if not exists credentials (
        id bigserial primary key,
        owner_id char(26) not null references users (id), -- your user/org id
        provider text not null default 'github',
        token_type text not null, -- 'oauth_refresh' | 'oauth_access' | 'app_private_key'
        ciphertext bytea not null, -- encrypted token
        token_nonce bytea not null, -- nonce used for token encryption
        wrapped_dek bytea not null, -- DEK encrypted with KEK
        dek_nonce bytea not null, -- nonce used for DEK wrapping
        created_at timestamptz not null default now (),
        last_used_at timestamptz,
        expires_at timestamptz
    );

create unique index credentials_unique on credentials (owner_id, token_type);

create index credentials_owner_id_idx on credentials (owner_id);

-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
drop index if exists credentials_unique;

drop index if exists credentials_owner_id_idx;

drop table if exists credentials;

-- +goose StatementEnd