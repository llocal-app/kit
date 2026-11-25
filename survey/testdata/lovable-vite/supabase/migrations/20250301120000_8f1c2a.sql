-- Profiles for each signed-in person
create table public.profiles (
  id uuid not null references auth.users on delete cascade,
  full_name text,
  created_at timestamp with time zone not null default now(),
  primary key (id)
);

alter table public.profiles enable row level security;

create policy "Users can view their own profile" on public.profiles
  for select using (auth.uid() = id);

create table public.jobs (
  id uuid primary key default gen_random_uuid(),
  user_id uuid not null references auth.users (id),
  customer text not null,
  address text not null,
  status text not null default 'new' check (status in ('new', 'quoted', 'won', 'lost')),
  quote_cents integer,
  created_at timestamp with time zone not null default now()
);

alter table public.jobs enable row level security;

create policy "Own jobs" on public.jobs for all using (auth.uid() = user_id);

insert into storage.buckets (id, name, public) values ('job-photos', 'job-photos', false);
