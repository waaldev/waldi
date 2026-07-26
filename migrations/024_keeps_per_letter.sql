-- A post kept from the feed and a letter kept about that same post are now
-- tracked independently: keeping one no longer marks the other as kept, and
-- both can appear on the shelf at once. This replaces the old one-row-per
-- (user, post) design with a surrogate id, plus two partial unique indexes
-- that each scope uniqueness to what they represent: at most one "plain"
-- keep per post (no letter), and at most one keep per letter.
alter table keeps drop constraint keeps_pkey;
alter table keeps add column id bigserial;
alter table keeps add primary key (id);

create unique index keeps_user_post_unletter_idx on keeps (user_id, post_id) where source_letter_id is null;
create unique index keeps_letter_idx on keeps (source_letter_id) where source_letter_id is not null;
