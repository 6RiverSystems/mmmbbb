ALTER TABLE topics
	ADD COLUMN live boolean CHECK (live != FALSE);

UPDATE
	topics
SET
	live = TRUE
WHERE
	deleted_at IS NULL;

ALTER TABLE topics
	ADD CONSTRAINT live_true_or_null CHECK (live != FALSE),
	ADD CONSTRAINT live_or_deleted CHECK ((live IS NOT NULL OR deleted_at IS NOT NULL) AND NOT (live IS
	NOT NULL AND deleted_at IS NOT NULL));

DROP INDEX topic_name_deleted_at;

CREATE UNIQUE INDEX topic_name_live ON topics (name, live);

CREATE INDEX topic_deleted_at ON topics (deleted_at);

ALTER TABLE subscriptions
	ADD COLUMN live boolean CHECK (live != FALSE);

UPDATE
	subscriptions
SET
	live = TRUE
WHERE
	deleted_at IS NULL;

ALTER TABLE subscriptions
	ADD CONSTRAINT live_true_or_null CHECK (live != FALSE),
	ADD CONSTRAINT live_or_deleted CHECK ((live IS NOT NULL OR deleted_at IS NOT NULL) AND NOT (live IS
	NOT NULL AND deleted_at IS NOT NULL));

CREATE UNIQUE INDEX subscription_name_live ON subscriptions (name, live);

DROP INDEX subscription_name;
