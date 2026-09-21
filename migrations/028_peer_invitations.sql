alter table invitations
  add column invited_by_user_id bigint references users(id) on delete cascade,
  add column first_post_notified_at timestamptz;

create index invitations_invited_by_idx on invitations (invited_by_user_id);
create index invitations_used_by_idx on invitations (used_by_user_id);

alter table users add column stranger_hold boolean not null default false;
