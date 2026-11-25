create table vendors (
  id bigint generated always as identity primary key,
  name text not null,
  phone text
);

create table purchase_orders (
  id bigint generated always as identity primary key,
  vendor_id bigint not null references vendors (id),
  total numeric(10, 2) not null,
  ordered_on date not null default current_date
);

alter table vendors enable row level security;
alter table purchase_orders enable row level security;
