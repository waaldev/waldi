alter table digests add column kind text not null default 'writer';
update digests set kind = 'writer' where kind is null;
alter table digests alter column kind drop default;
alter table digests add constraint digests_kind_check check (kind in ('writer', 'reader'));

create index digests_user_kind_sent_idx on digests (user_id, kind, sent_at);
