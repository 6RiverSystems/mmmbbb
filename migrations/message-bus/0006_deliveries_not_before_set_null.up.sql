ALTER TABLE deliveries
	DROP CONSTRAINT deliveries_not_before_id_fkey;

ALTER TABLE deliveries
	ADD CONSTRAINT deliveries_not_before_id_fkey FOREIGN KEY (not_before_id) REFERENCES deliveries (id) ON DELETE SET NULL;
