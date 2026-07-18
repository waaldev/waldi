alter table wildcards drop constraint wildcards_pkey;
alter table wildcards add primary key (user_id, date, post_id);
create unique index wildcards_active_idx on wildcards (user_id, date) where not skipped;
