alter table users
  add column display_name text not null default '',
  add column bio text not null default '',
  add column blog_lang text not null default 'fa',
  add constraint users_blog_lang_valid check (blog_lang in ('fa', 'en'));

update users set blog_lang = locale where blog_lang = 'fa' and locale = 'en';
