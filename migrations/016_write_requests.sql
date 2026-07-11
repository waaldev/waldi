create table write_requests (
  id bigserial primary key,
  user_id bigint not null references users(id) on delete cascade,
  blog_link text not null default '',
  note text not null default '',
  created_at timestamptz not null default now()
);

create index write_requests_user_idx on write_requests (user_id);
