CREATE TABLE schemaversion (
  version TEXT NOT NULL
);

-- Write discovered networks here.
-- Hashable columns should match that of `registry` below.
CREATE TABLE lobby (
  -- BEGIN HASHABLE --
  uuid TEXT NOT NULL PRIMARY KEY,
  name TEXT NOT NULL,
  description TEXT NOT NULL,
  tags TEXT NOT NULL,
  links TEXT NOT NULL,
  location TEXT NOT NULL,
  rendezvous TEXT NOT NULL,
  creator TEXT NOT NULL,
  created INT NOT NULL,
  -- END HASHABLE --

  signature BYTES NOT NULL -- hash of hashable columns above, verifiable with creator pubkey
);

-- Write network stats here (from discovery or peer upload)
CREATE TABLE stats (
  uuid TEXT NOT NULL PRIMARY KEY,
  population INT NOT NULL,
  completions_last7days INT NOT NULL,
  records INT NOT NULL,
  idle_records INT NOT NULL,
  avg_completion_time INT NOT NULL
);

-- Write records to publish here. Hashable columns should match
-- that of `lobby` above.
CREATE TABLE registry (
  -- BEGIN HASHABLE --
  uuid TEXT NOT NULL PRIMARY KEY,
  name TEXT NOT NULL,
  description TEXT NOT NULL,
  tags TEXT NOT NULL,
  links TEXT NOT NULL,
  location TEXT NOT NULL,
  rendezvous TEXT NOT NULL,
  creator TEXT NOT NULL,
  created INT NOT NULL,
  -- END HASHABLE --

  signature BYTES NOT NULL -- hash of hashable columns above, verifiable with creator pubkey
);

