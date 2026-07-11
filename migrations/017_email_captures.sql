create table email_captures (
  id bigserial primary key,
  email text not null,
  source_username text not null default '',
  source_post_id bigint references posts(id) on delete set null,
  created_at timestamptz not null default now()
);

create unique index email_captures_email_idx on email_captures (email);
