create table invitations (
  id bigserial primary key,
  code text not null unique,
  note text not null default '',
  created_at timestamptz not null default now(),
  used_at timestamptz,
  used_by_user_id bigint references users(id) on delete set null
);

create index invitations_unused_idx on invitations (code) where used_at is null;
