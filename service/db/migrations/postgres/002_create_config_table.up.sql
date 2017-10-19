CREATE TABLE IF NOT EXISTS config
  (instance varchar(255) NOT NULL,
   config   text NOT NULL,
   stamp    timestamp with time zone NOT NULL)
