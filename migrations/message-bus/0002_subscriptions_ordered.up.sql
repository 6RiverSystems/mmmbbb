ALTER TABLE subscriptions
	ADD COLUMN ordered_delivery boolean NOT NULL DEFAULT FALSE;
