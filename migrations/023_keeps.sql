create table keeps (
  user_id bigint not null references users(id) on delete cascade,
  post_id bigint not null references posts(id) on delete cascade,
  source_letter_id bigint references letters(id) on delete set null,
  created_at timestamptz not null default now(),
  primary key (user_id, post_id)
);

create index keeps_user_created_idx on keeps (user_id, created_at desc);
