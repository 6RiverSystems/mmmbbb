-- supports the query for "get messages to deliver"
create index id_delivery_sub_attempt_ready on deliveries (subscription_id, attempt_at) where (completed_at is null);
