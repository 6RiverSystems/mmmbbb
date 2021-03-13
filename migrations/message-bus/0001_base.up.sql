CREATE TABLE topics (
	id uuid NOT NULL,
	name text NOT NULL,
	created_at timestamptz NOT NULL,
	deleted_at timestamptz NULL,
	PRIMARY KEY (id)
);

CREATE INDEX topic_name_deleted_at ON topics (name, deleted_at);

--
CREATE TABLE subscriptions (
	id uuid NOT NULL,
	name text NOT NULL,
	created_at timestamptz NOT NULL,
	deleted_at timestamptz NULL,
	message_ttl interval NOT NULL,
	topic_id uuid NOT NULL,
	PRIMARY KEY (id),
	FOREIGN KEY (topic_id) REFERENCES topics (id)
);

CREATE INDEX subscription_name ON subscriptions (name);

CREATE INDEX subscription_deleted_at ON subscriptions (deleted_at);

--
CREATE TABLE messages (
	id uuid NOT NULL,
	payload jsonb NOT NULL,
	published_at timestamptz NOT NULL,
	topic_id uuid NOT NULL,
	PRIMARY KEY (id),
	FOREIGN KEY (topic_id) REFERENCES topics (id)
);

CREATE INDEX message_published_at ON messages (published_at);

--
CREATE TABLE deliveries (
	id uuid NOT NULL,
	attempt_at timestamptz NOT NULL,
	last_attempted_at timestamptz NULL,
	attempts integer NOT NULL,
	completed_at timestamptz NULL,
	expires_at timestamptz NOT NULL,
	message_id uuid NOT NULL,
	subscription_id uuid NOT NULL,
	not_before_id uuid NULL,
	PRIMARY KEY (id),
	FOREIGN KEY (message_id) REFERENCES messages (id),
	FOREIGN KEY (subscription_id) REFERENCES subscriptions (id),
	FOREIGN KEY (not_before_id) REFERENCES deliveries (id)
);

CREATE INDEX delivery_attempt_at ON deliveries (attempt_at);

CREATE INDEX delivery_expires_at ON deliveries (expires_at);

CREATE INDEX delivery_not_before_id ON deliveries (not_before_id);

CREATE INDEX delivery_subscription_id ON deliveries (subscription_id);

CREATE INDEX delivery_message_id ON deliveries (message_id);
