alter table users
  add column email_verified_at timestamptz,
  add column email_verify_token text,
  add column email_verify_sent_at timestamptz,
  add column password_reset_token text,
  add column password_reset_expires_at timestamptz;

update users set email_verified_at = created_at where email_verified_at is null;

create unique index users_email_verify_token_idx
  on users (email_verify_token)
  where email_verify_token is not null;

create unique index users_password_reset_token_idx
  on users (password_reset_token)
  where password_reset_token is not null;
