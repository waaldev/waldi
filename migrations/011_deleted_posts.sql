create table deleted_posts (
    user_id bigint not null references users(id) on delete cascade,
    slug text not null,
    deleted_at timestamptz not null default now(),
    primary key (user_id, slug)
);
