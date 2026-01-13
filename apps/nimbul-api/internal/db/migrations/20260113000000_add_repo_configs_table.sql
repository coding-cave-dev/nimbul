-- +goose Up
-- +goose StatementBegin
create table
    if not exists repo_configs (
        id char(26) primary key, -- ULID
        owner_id char(26) not null references users (id),
        provider text not null default 'github',
        repo_owner text not null, -- GitHub username/org
        repo_name text not null,
        repo_full_name text not null, -- owner/repo
        repo_clone_url text not null,
        dockerfile_path text not null,
        webhook_secret text not null, -- For verifying webhook payloads
        webhook_id bigint, -- GitHub webhook ID
        created_at timestamptz not null default now (),
        updated_at timestamptz not null default now ()
    );

create index repo_configs_owner_id_idx on repo_configs (owner_id);

create index repo_configs_provider_idx on repo_configs (provider);

create unique index repo_configs_repo_unique on repo_configs (owner_id, repo_full_name);

-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
drop index if exists repo_configs_repo_unique;

drop index if exists repo_configs_provider_idx;

drop index if exists repo_configs_owner_id_idx;

drop table if exists repo_configs;

-- +goose StatementEnd