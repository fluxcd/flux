UPDATE config SET instance = "<default-instance-id>" WHERE instance = "DEFAULT";

CREATE UNIQUE INDEX config_pk ON config (instance);
