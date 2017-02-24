CREATE TABLE IF NOT EXISTS events (
    instance_id  string  NOT NULL,
    type         string  NOT NULL,
    started_at   time    NOT NULL DEFAULT now(),
    ended_at     time    NOT NULL DEFAULT now(),
    log_level    string  NOT NULL DEFAULT "info",
    message      string  NOT NULL DEFAULT "",
    metadata     string,
);

CREATE TABLE IF NOT EXISTS event_service_ids (
    event_id    int     NOT NULL,
    service_id  string  NOT NULL,
);

INSERT INTO events
  (instance_id, type, started_at, ended_at)
  SELECT
    instance, "automate", stamp as started_at, stamp as ended_at
    FROM history
    WHERE message = "Automation enabled.";

INSERT INTO events
  (instance_id, type, started_at, ended_at)
  SELECT
    instance, "deautomate", stamp as started_at, stamp as ended_at
    FROM history
    WHERE message = "Automation disabled.";

INSERT INTO events
  (instance_id, type, started_at, ended_at)
  SELECT
    instance, "lock", stamp as started_at, stamp as ended_at
    FROM history
    WHERE message = "Service locked.";

INSERT INTO events
  (instance_id, type, started_at, ended_at)
  SELECT
    instance, "unlock", stamp as started_at, stamp as ended_at
    FROM history
    WHERE message = "Service unlocked.";

-- Move any existing releases over to the new table. They won"t have full
-- metadata, and will have all our weird historical message formats.
INSERT INTO events
  (instance_id, type, started_at, ended_at, message)
  SELECT
    instance, string(namespace) + "/" + string(service), "release", stamp as started_at, stamp as ended_at, message
    FROM history
    WHERE message LIKE "(Release|Regrade)" ;

INSERT INTO event_service_ids
  (event_id, service_id)
  SELECT
    id(), string(h.namespace) + "/" + string(h.service)
    FROM events as e, history as h
    WHERE e.started_at = h.stamp
    AND e.instance_id = h.instance;
