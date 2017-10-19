ALTER TABLE history
  ADD instance varchar(255),
  ADD id serial NOT NULL;

UPDATE history SET instance = '<default-instance-id>';

ALTER TABLE history
  ALTER COLUMN instance SET NOT NULL,
  ADD PRIMARY KEY (id);
