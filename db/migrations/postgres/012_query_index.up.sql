-- Optimise per-instance history queries
CREATE INDEX events_instance_id_started_at_desc_idx ON events USING btree(instance_id, started_at DESC);

-- These optimise queries performed by flux 0.3.x
CREATE INDEX jobs_instance_id_idx ON jobs USING btree(instance_id);
CREATE INDEX jobs_busy_instance_id_idx ON jobs USING btree(instance_id) WHERE claimed_at IS NOT NULL AND finished_at IS NULL;
CREATE INDEX jobs_queue_order_idx ON jobs USING btree((-1 * priority), scheduled_at, submitted_at) WHERE finished_at IS NULL AND claimed_at IS NULL;
CREATE INDEX jobs_instance_id_unfinished_idx ON jobs USING btree(instance_id) WHERE finished_at IS NULL;
