UPDATE config SET instance = '<default-instance-id>' WHERE instance = 'DEFAULT';

ALTER TABLE config ADD PRIMARY KEY (instance);
