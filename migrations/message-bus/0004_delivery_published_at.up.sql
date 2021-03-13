ALTER TABLE deliveries
	ADD COLUMN published_at timestamptz NULL;

UPDATE
	deliveries
SET
	published_at = (
		SELECT
			published_at
		FROM
			messages
		WHERE
			id = deliveries.message_id);

ALTER TABLE deliveries
	ALTER COLUMN published_at SET NOT NULL;

CREATE INDEX delivery_published_at ON deliveries (published_at);
