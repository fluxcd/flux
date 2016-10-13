CREATE TABLE IF NOT EXISTS release_jobs (
    release_id   string NOT NULL,
    instance_id  string NOT NULL,
    submitted_at time   NOT NULL,
    claimed_at   time,
    job          string NOT NULL,
)
