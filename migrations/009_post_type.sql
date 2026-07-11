alter table posts add column type text not null default 'post';
alter table posts add constraint posts_type_valid check (type in ('post', 'page'));
alter table posts add column page_position integer;
