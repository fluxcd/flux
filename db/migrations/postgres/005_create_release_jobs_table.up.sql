CREATE TABLE IF NOT EXISTS release_jobs (
    PRIMARY KEY (release_id),
    release_id   UUID                         NOT NULL,
    instance_id  varchar(255)                 NOT NULL,
    spec         text                         NOT NULL,
    submitted_at timestamp without time zone  NOT NULL,
    claimed_at   timestamp without time zone,
    heartbeat_at timestamp without time zone,
    finished_at  timestamp without time zone,
    log          text                         NOT NULL,
    status       text                         NOT NULL,
    success      boolean
)
