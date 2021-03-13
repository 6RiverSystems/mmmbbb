ALTER TABLE subscriptions
	ADD COLUMN ttl interval NULL,
	ADD COLUMN expires_at timestamptz NULL;

UPDATE
	subscriptions
SET
	ttl = '30 days'::interval,
	expires_at = now() + '30 days'::interval;

ALTER TABLE subscriptions
	ALTER COLUMN ttl SET NOT NULL,
	ALTER COLUMN expires_at SET NOT NULL;
