ALTER TABLE history ADD instance string NOT NULL DEFAULT "<default-instance-id>";

CREATE UNIQUE INDEX history_pk ON history (id());
