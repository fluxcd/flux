CREATE TABLE IF NOT EXISTS events (
    PRIMARY KEY (id),
    id           bigserial,
    instance_id  text                      NOT NULL,
    service_ids  text[]                    NOT NULL,
    type         text                      NOT NULL,
    started_at   timestamp with time zone  NOT NULL DEFAULT now(),
    ended_at     timestamp with time zone  NOT NULL DEFAULT now(),
    log_level    text                      NOT NULL DEFAULT 'info',
    message      text                      NOT NULL DEFAULT '',
    metadata     jsonb
);

INSERT INTO events
  (instance_id, service_ids, type, started_at, ended_at)
  SELECT
    instance, ARRAY[concat(namespace, '/', service)], 'automate', stamp, stamp
    FROM history
    WHERE message = 'Automation enabled.';

INSERT INTO events
  (instance_id, service_ids, type, started_at, ended_at)
  SELECT
    instance, ARRAY[concat(namespace, '/', service)], 'deautomate', stamp, stamp
    FROM history
    WHERE message = 'Automation disabled.';

INSERT INTO events
  (instance_id, service_ids, type, started_at, ended_at)
  SELECT
    instance, ARRAY[concat(namespace, '/', service)], 'lock', stamp, stamp
    FROM history
    WHERE message = 'Service locked.';

INSERT INTO events
  (instance_id, service_ids, type, started_at, ended_at)
  SELECT
    instance, ARRAY[concat(namespace, '/', service)], 'unlock', stamp, stamp
    FROM history
    WHERE message = 'Service unlocked.';

-- Move any existing releases over to the new table. They won't have full
-- metadata, and will have all our weird historical message formats.
INSERT INTO events
  (instance_id, service_ids, type, started_at, ended_at, message)
  SELECT
    instance, ARRAY[concat(namespace, '/', service)], 'release', stamp, stamp, message
    FROM history
    WHERE message LIKE '%Release%' OR message LIKE '%Regrade%' ;
