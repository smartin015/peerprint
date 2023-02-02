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
		FOREIGN KEY (uuid, signer) references records(uuid, signer) 
		ON DELETE CASCADE
);

CREATE TABLE events (
  event TEXT NOT NULL,
  details TEXT NOT NULL,
  timestamp INT NOT NULL
);

CREATE TABLE peers (
  peer TEXT NOT NULL PRIMARY KEY,
	first_seen INT NOT NULL,
	last_seen INT NOT NULL
);
 
CREATE TABLE census (
  peer TEXT NOT NULL,
  timestamp INT NOT NULL,

  PRIMARY KEY (peer, timestamp)
);

