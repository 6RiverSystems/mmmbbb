-- Copyright (c) 2021 6 River Systems
--
-- Permission is hereby granted, free of charge, to any person obtaining a copy of
-- this software and associated documentation files (the "Software"), to deal in
-- the Software without restriction, including without limitation the rights to
-- use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of
-- the Software, and to permit persons to whom the Software is furnished to do so,
-- subject to the following conditions:
--
-- The above copyright notice and this permission notice shall be included in all
-- copies or substantial portions of the Software.
--
-- THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
-- IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS
-- FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR
-- COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER
-- IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN
-- CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.

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
