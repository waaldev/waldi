create table app_settings (
  id boolean primary key default true,
  wildcard_impression_floor integer not null default 100,
  constraint app_settings_singleton check (id)
);

insert into app_settings (id) values (true);
