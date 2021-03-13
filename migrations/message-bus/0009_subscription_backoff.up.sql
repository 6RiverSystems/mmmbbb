ALTER TABLE subscriptions
	ADD COLUMN min_backoff interval NULL,
	ADD COLUMN max_backoff interval NULL;
