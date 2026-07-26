-- A reply is just another letters row addressed back to the original
-- sender, inheriting the same post_id. in_reply_to always points at the
-- thread root (the first, non-reply letter), never the immediate parent,
-- so a thread is the root row plus every row with in_reply_to = root.id.
alter table letters add column in_reply_to bigint references letters(id);
alter table letters add column closed_at timestamptz;

create index letters_in_reply_to_idx on letters (in_reply_to);
