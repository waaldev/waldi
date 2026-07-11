alter table sessions
  add column bridge_token text;

create unique index sessions_bridge_token_idx
  on sessions (bridge_token)
  where bridge_token is not null;
