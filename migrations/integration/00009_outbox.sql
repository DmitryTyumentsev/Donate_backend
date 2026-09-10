-- goose Up
drop table if exists integration.outbox;

create table if not exists integration.fixations_outbox(
id bigserial primary key,
amo_id int not null,
name varchar(64) not null,
created_at timestamp default now(),
updated_at timestamp default now(),
created_by int,
updated_by int,
responsible_user_id int,
pipeline_id int,
price int
);
create unique index idx_fixations_outbox_amo_id on integration.fixations_outbox(amo_id);

-- goose Down
drop table if exists integration.fixations_outbox;