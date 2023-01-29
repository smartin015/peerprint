CREATE TABLE schemaversion (
  version TEXT NOT NULL
);

CREATE TABLE records (
  uuid TEXT NOT NULL,
  tags TEXT NOT NULL,
  approver TEXT NOT NULL,
  manifest TEXT NOT NULL,
  created INT NOT NULL,

  num INT NOT NULL,
  den INT NOT NULL,
  gen REAL NOT NULL,

  signer BLOB NOT NULL,
  signature BLOB NOT NULL,

  PRIMARY KEY (uuid, signer)
);

CREATE TABLE completions (
  uuid TEXT NOT NULL,
  completer TEXT NOT NULL,
  completer_state BLOB NOT NULL,
  timestamp INT NOT NULL,

  signer TEXT NOT NULL,
  signature BLOB NOT NULL,

	CONSTRAINT completions_pk 
		PRIMARY KEY (uuid, signer),

	CONSTRAINT completions_uuid_fk 
		FOREIGN KEY (uuid) references records(uuid) 
		ON DELETE CASCADE,

	CONSTRAINT completions_signer_fk 
		FOREIGN KEY (signer) references records(signer) 
		ON DELETE CASCADE
);

CREATE TABLE events (
  event TEXT NOT NULL,
  details TEXT NOT NULL,
  timestamp INT NOT NULL
);

CREATE TABLE workability (
  uuid TEXT NOT NULL,
  origin TEXT NOT NULL,
  timestamp INT NOT NULL,
  workability REAL NOT NULL,

	CONSTRAINT workability_pk 
		PRIMARY KEY (uuid, origin),

  CONSTRAINT workability_uuid_fk
    FOREIGN KEY (uuid) references records(uuid)
    ON DELETE CASCADE,

	CONSTRAINT workability_signer_fk 
		FOREIGN KEY (origin) references records(signer) 
		ON DELETE CASCADE
);

CREATE TABLE trust (
  peer TEXT NOT NULL PRIMARY KEY,
  -- worker_trust is how likely we think this peer
  -- will complete our Record
  worker_trust REAL NOT NULL DEFAULT 0,
  -- reward_trust is how likely we think this peer
  -- will reward us if we complete their Record
  reward_trust REAL NOT NULL DEFAULT 0,
	first_seen INT NOT NULL,
	last_seen INT NOT NULL,
  timestamp INT NOT NULL
);
 
CREATE TABLE census (
  peer TEXT NOT NULL,
  timestamp INT NOT NULL,

  PRIMARY KEY (peer, timestamp)
);

