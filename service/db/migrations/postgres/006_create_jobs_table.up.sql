CREATE TABLE IF NOT EXISTS jobs (
    PRIMARY KEY (id),
    instance_id  text                      NOT NULL,
    id           UUID                      NOT NULL,
    queue        text                      NOT NULL DEFAULT 'default',
    method       text                      NOT NULL,
    params       jsonb,
    scheduled_at timestamp with time zone  NOT NULL DEFAULT now(),
    priority     integer                   NOT NULL DEFAULT 0,
    key          text,
    submitted_at timestamp with time zone  NOT NULL DEFAULT now(),
    claimed_at   timestamp with time zone,
    heartbeat_at timestamp with time zone,
    finished_at  timestamp with time zone,
    log          text                      NOT NULL,
    status       text                      NOT NULL,
    done         boolean,
    success      boolean
);

-- Move any existing jobs over to the new table. Only do unclaimed ones, as we
-- assume any others are currently in-progress.
INSERT INTO jobs
  (instance_id, id, method, params, scheduled_at, submitted_at, log, status)
  SELECT
    instance_id, release_id, 'release', spec::jsonb, submitted_at, submitted_at, log, status
    FROM release_jobs
    WHERE claimed_at IS NULL;
