alter table users alter column can_write set default true;
update users set can_write = true where not can_write;
