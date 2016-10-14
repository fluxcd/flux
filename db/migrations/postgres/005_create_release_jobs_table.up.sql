CREATE TABLE IF NOT EXISTS release_jobs (
    PRIMARY KEY (release_id),
    release_id   UUID                                      NOT NULL,
    instance_id  varchar(255)                              NOT NULL,
    submitted_at timestamp without time zone DEFAULT now() NOT NULL,
    claimed_at   timestamp without time zone,
    finished_at  timestamp without time zone,
    job          text                                      NOT NULL,
)
