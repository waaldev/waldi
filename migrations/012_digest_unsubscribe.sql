alter table users
  add column digest_unsubscribe_token text,
  add column digest_unsubscribed_at timestamptz;

create unique index users_digest_unsubscribe_token_idx
  on users (digest_unsubscribe_token)
  where digest_unsubscribe_token is not null;
