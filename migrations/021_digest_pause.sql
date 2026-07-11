alter table users
  add column last_active_at timestamptz not null default now(),
  add column digest_paused_at timestamptz;
