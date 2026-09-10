create index posts_published_by_user_idx
  on posts (user_id, published_at desc, id desc)
  where status = 'published' and type = 'post';

create index posts_drafts_by_user_idx
  on posts (user_id, updated_at desc, id desc)
  where status = 'draft' and type = 'post';

create index letters_post_idx on letters (post_id);
