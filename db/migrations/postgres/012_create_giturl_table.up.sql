CREATE TABLE IF NOT EXISTS giturl (
  instance varchar(255) NOT NULL,
  giturl   text NOT NULL,
  stamp    timestamp with time zone NOT NULL
)
