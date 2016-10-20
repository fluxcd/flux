CREATE TABLE IF NOT EXISTS release_jobs (
    release_id   string NOT NULL,
    instance_id  string NOT NULL,
    spec         string NOT NULL,
    submitted_at time   NOT NULL,
    claimed_at   time,
    heartbeat_at time,
    finished_at  time,
    log          string NOT NULL,
    status       string NOT NULL,
    success      bool,
)
