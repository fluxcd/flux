CREATE TABLE IF NOT EXISTS jobs (
    instance_id  string NOT NULL,
    id           string NOT NULL,
    queue        string NOT NULL DEFAULT "default",
    method       string NOT NULL,
    params       string,
    scheduled_at time   NOT NULL,
    priority     int    NOT NULL DEFAULT 0,
    key          string,
    submitted_at time   NOT NULL,
    claimed_at   time,
    heartbeat_at time,
    finished_at  time,
    log          string NOT NULL,
    status       string NOT NULL,
    done         bool,
    success      bool,
);

-- Move any existing jobs over to the new table. Only do unclaimed ones, as we
-- assume any others are currently in-progress.
INSERT INTO jobs
  (instance_id, id, method, params, scheduled_at, submitted_at, log, status)
  SELECT
    instance_id, release_id, "release", spec, submitted_at AS scheduled_at, submitted_at, log, status
    FROM release_jobs
    WHERE claimed_at IS NULL;
