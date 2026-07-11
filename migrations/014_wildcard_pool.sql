create table wildcard_pool (
  id bigserial primary key,
  post_id bigint not null references posts(id) on delete cascade,
  added_at timestamptz not null default now(),
  unique (post_id)
);
