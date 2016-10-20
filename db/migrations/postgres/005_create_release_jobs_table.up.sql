CREATE TABLE IF NOT EXISTS release_jobs (
    PRIMARY KEY (release_id),
    release_id   UUID                      NOT NULL,
    instance_id  varchar(255)              NOT NULL,
    spec         text                      NOT NULL,
    submitted_at timestamp with time zone  NOT NULL,
    claimed_at   timestamp with time zone,
    heartbeat_at timestamp with time zone,
    finished_at  timestamp with time zone,
    log          text                      NOT NULL,
    status       text                      NOT NULL,
    success      boolean
)
