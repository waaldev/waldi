-- Tracks the reader digest sent to anonymous (not-yet-registered) captured
-- emails: one row per email, holding its unsubscribe token and last send
-- time. Deleted once that address signs up (their capture rows are adopted
-- into real follows and this bookkeeping is no longer needed).
create table email_capture_digests (
  email text primary key,
  unsubscribe_token text not null unique,
  sent_at timestamptz
);
