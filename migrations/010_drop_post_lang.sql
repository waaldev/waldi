alter table posts drop constraint if exists posts_dir_valid;
alter table posts drop column if exists lang;
alter table posts drop column if exists dir;
