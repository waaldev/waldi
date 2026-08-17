-- Assignment is delivery intent, not an impression. Track the first actual
-- open separately so dormant accounts do not enter the completion denominator.
alter table wildcards add column opened_at timestamptz;

-- Recover opens whose original impression source proves they came through a
-- wildcard link. Reader/post impression dedup means direct-first cases cannot
-- be reconstructed; those safely remain outside the denominator.
with historical as (
    select
        w.user_id,
        w.post_id,
        w.date,
        min(i.created_at) as opened_at
    from wildcards as w
    inner join impressions as i
        on
            w.post_id = i.post_id
            and i.reader_key = 'user:' || w.user_id
            and i.source = 'wildcard'
            and w.date <= i.created_at
            and i.created_at < w.date + interval '1 day'
    group by w.user_id, w.post_id, w.date
)

update wildcards as w
set opened_at = historical.opened_at
from historical
where
    w.user_id = historical.user_id
    and w.post_id = historical.post_id
    and w.date = historical.date;
