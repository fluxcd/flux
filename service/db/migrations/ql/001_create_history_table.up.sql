CREATE TABLE IF NOT EXISTS history
       (namespace string NOT NULL,
        service   string NOT NULL,
        message   string NOT NULL,
        stamp     time NOT NULL)
