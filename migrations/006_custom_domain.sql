alter table users
  add column custom_domain text,
  add column custom_domain_token text,
  add column custom_domain_verified_at timestamptz;

alter table users
  add constraint users_custom_domain_format
    check (custom_domain is null or custom_domain ~ '^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?(\.[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?)+$');

create unique index users_custom_domain_idx
  on users (custom_domain)
  where custom_domain is not null;
