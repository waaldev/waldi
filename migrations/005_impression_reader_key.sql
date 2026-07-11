alter table impressions add column if not exists reader_key text;

update impressions
set reader_key = coalesce('user:' || user_id::text, 'legacy:' || id::text)
where reader_key is null;

-- Keep one row per (post_id, reader_key); merge reading stats from duplicates first.
with keepers as (
  select post_id, reader_key, min(id) as keep_id
  from impressions
  group by post_id, reader_key
),
dupes as (
  select i.id as dupe_id, k.keep_id
  from impressions i
  join keepers k on k.post_id = i.post_id and k.reader_key = i.reader_key
  where i.id <> k.keep_id
)
insert into readings (impression_id, max_scroll_pct, dwell_seconds, completed, updated_at)
select d.keep_id, r.max_scroll_pct, r.dwell_seconds, r.completed, r.updated_at
from dupes d
join readings r on r.impression_id = d.dupe_id
on conflict (impression_id) do update set
  max_scroll_pct = greatest(readings.max_scroll_pct, excluded.max_scroll_pct),
  dwell_seconds = greatest(readings.dwell_seconds, excluded.dwell_seconds),
  completed = readings.completed or excluded.completed,
  updated_at = greatest(readings.updated_at, excluded.updated_at);

delete from impressions i
using impressions j
where i.post_id = j.post_id
  and i.reader_key = j.reader_key
  and i.id > j.id;

alter table impressions alter column reader_key set not null;

create unique index if not exists impressions_post_reader_key_idx on impressions (post_id, reader_key);
