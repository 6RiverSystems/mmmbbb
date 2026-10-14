-- Copyright (c) 2026 6 River Systems
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
-- blocked is true while a delivery in an ordered subscription is waiting on its
-- not_before predecessor, so finding ready deliveries can use an index instead
-- of checking the predecessor of every pending delivery
alter table
	deliveries
add
	column blocked boolean not null default false;

update
	deliveries
set
	blocked = true
where
	completed_at is null
	and not_before_id in (
		select
			id
		from
			deliveries
		where
			completed_at is null
	);

create index id_delivery_sub_attempt_unblocked on deliveries (subscription_id, attempt_at)
where
	(
		completed_at is null
		and not blocked
	);

create index id_delivery_sub_published_at on deliveries (subscription_id, published_at);