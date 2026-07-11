-- A visitor may subscribe from more than one writer's post before ever
-- signing up; each should be remembered so signup can follow all of them.
drop index email_captures_email_idx;
create unique index email_captures_email_source_idx on email_captures (email, source_username);
