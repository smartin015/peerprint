CREATE TABLE schemaversion (
  version TEXT NOT NULL
);

CREATE TABLE events (
  event TEXT NOT NULL,
  details TEXT NOT NULL,
  timestamp INT NOT NULL
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

CREATE TABLE workability (
  uuid TEXT NOT NULL PRIMARY KEY,
  timestamp INT NOT NULL,
  workability REAL NOT NULL
);

CREATE TABLE trust (
  peer TEXT NOT NULL PRIMARY KEY,
  trust REAL NOT NULL DEFAULT 0,
  timestamp INT NOT NULL
);

CREATE TABLE census (
  peer TEXT NOT NULL PRIMARY KEY,
  earliest INT NOT NULL,
  latest INT NOT NULL
);

CREATE TABLE completions (
  uuid TEXT NOT NULL,
  completer TEXT NOT NULL,
  completer_state BLOB NOT NULL,
  timestamp INT NOT NULL,

  signer TEXT NOT NULL,
  signature BLOB NOT NULL,

  PRIMARY KEY (uuid, signer)
);
