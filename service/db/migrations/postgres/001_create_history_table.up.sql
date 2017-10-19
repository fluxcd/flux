CREATE TABLE IF NOT EXISTS history
       (namespace text NOT NULL,
       	service   text NOT NULL,
	message   text NOT NULL,
	stamp     timestamp with time zone NOT NULL)
