alter table users
  add column can_write boolean not null default false;

-- Grandfather in every existing account: under the old invite-only model,
-- every account that exists today was already a writer.
update users set can_write = true;

alter table users alter column can_write set default false;
