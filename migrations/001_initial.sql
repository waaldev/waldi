create table users (
  id bigserial primary key,
  username text not null unique,
  email text not null unique,
  password_hash text not null,
  locale text not null default 'fa',
  created_at timestamptz not null default now(),
  constraint users_username_format check (username ~ '^[a-z0-9-]{3,32}$')
);

create table sessions (
  token text primary key,
  user_id bigint not null references users(id) on delete cascade,
  expires_at timestamptz not null
);

create table posts (
  id bigserial primary key,
  user_id bigint not null references users(id) on delete cascade,
  title text not null,
  slug text not null,
  doc jsonb not null,
  html text not null,
  status text not null,
  lang text not null default 'fa',
  dir text not null default 'rtl',
  word_count integer not null default 0,
  published_at timestamptz,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  unique (user_id, slug),
  constraint posts_status_valid check (status in ('draft', 'published')),
  constraint posts_dir_valid check (dir in ('rtl', 'ltr'))
);

create table follows (
  follower_id bigint not null references users(id) on delete cascade,
  followee_id bigint not null references users(id) on delete cascade,
  source_post_id bigint references posts(id) on delete set null,
  created_at timestamptz not null default now(),
  primary key (follower_id, followee_id),
  constraint follows_not_self check (follower_id <> followee_id)
);

create table letters (
  id bigserial primary key,
  post_id bigint not null references posts(id) on delete cascade,
  from_user bigint not null references users(id) on delete cascade,
  to_user bigint not null references users(id) on delete cascade,
  body text not null,
  created_at timestamptz not null default now(),
  read_at timestamptz
);

create table impressions (
  id bigserial primary key,
  post_id bigint not null references posts(id) on delete cascade,
  user_id bigint references users(id) on delete set null,
  source text not null,
  created_at timestamptz not null default now(),
  constraint impressions_source_valid check (source in ('feed', 'wildcard', 'direct'))
);

create index impressions_post_created_idx on impressions (post_id, created_at);

create table readings (
  impression_id bigint primary key references impressions(id) on delete cascade,
  max_scroll_pct integer not null default 0,
  dwell_seconds integer not null default 0,
  completed boolean not null default false,
  updated_at timestamptz not null default now(),
  constraint readings_scroll_valid check (max_scroll_pct between 0 and 100)
);

create table wildcards (
  user_id bigint not null references users(id) on delete cascade,
  post_id bigint not null references posts(id) on delete cascade,
  date date not null,
  skipped boolean not null default false,
  primary key (user_id, date)
);

create table digests (
  user_id bigint not null references users(id) on delete cascade,
  sent_at timestamptz not null,
  primary key (user_id, sent_at)
);

