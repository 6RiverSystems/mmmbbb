ALTER TABLE subscriptions
	ADD COLUMN max_delivery_attempts integer NULL CHECK (max_delivery_attempts >= 0),
	ADD COLUMN dead_letter_topic_id uuid REFERENCES topics ON DELETE SET NULL;
