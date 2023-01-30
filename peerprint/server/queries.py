import sqlite3
import peerprint.server.pkg.proto.state_pb2 as spb

class DBReader:
    def __init__(self, dbPath):
        # Open in read-only mode; all writes must
        # occur via the server.
        dbPath = 'file:' + dbPath + '?mode=ro'
        print("Connecting to ", dbPath)
        self.con = sqlite3.connect(dbPath)
        self.con.execute('pragma journal_mode=wal')

    def _toRecord(self, vals):
        if vals is None:
            return None
        r = spb.SignedRecord()
        r.record.uuid = vals[0]
        for v in vals[1].split(","):
            r.record.tags.append(v)
        r.record.approver = vals[2]
        r.record.manifest = vals[3]
        r.record.created = vals[4]
        r.record.rank.num = vals[5]
        r.record.rank.den = vals[6]
        r.record.rank.gen = vals[7]
        r.signature.signer = vals[8]
        r.signature.data = vals[9]
        return r

    def _toCompletion(self, vals):
        if vals is None:
            return None
        c = spb.SignedCompletion()
        c.completion.uuid = vals[0]
        c.completion.completer = vals[1]
        c.completion.completer_state = vals[2]
        c.completion.timestamp = vals[3]
        c.signature.signer = vals[4]
        c.signature.data = vals[5]
        return c

    def _all(self, *args):
        cur = self.con.cursor()
        try:
            return cur.execute(*args).fetchall()
        finally:
            cur.close()

    def _one(self, *args):
        cur = self.con.cursor()
        try:
            return cur.execute(*args).fetchone()
        finally:
            cur.close()

    def _singleton(self, *args):
        try:
            return self._one(*args)[0]
        except IndexError:
            return None

    def countRecords(self):
        return self._singleton("SELECT COUNT(*) FROM records")

    def getRecords(self):
        return [self._toRecord(r) for r in self._all("SELECT * FROM records")]



"""

// When receiving work, sponsor/nominate it
    if trust, err := s.s.GetWorkerTrust(peer); err != nil {
      return fmt.Errorf("GetTrust: %w", err)
    } else if trust > s.opts.TrustedWorkerThreshold {
      // TODO prevent another trusted peer from coming along and swiping the work - must check if record already has a sponsored worker
      s.l.Info("We nominate %s to complete (%s)", shorten(peer), c.Uuid)
      _, err := s.IssueCompletion(c, true)
      return err
    } else {
      return nil
    }

func (s *sqlite3) deleteRecordsUntilSignerCount(trustThresh float64, untilPeers int64) error {
  n := int64(0)
  if err := s.db.QueryRow(`SELECT COUNT(DISTINCT signer) FROM "records"`).Scan(&n); err != nil {
    return fmt.Errorf("Count record signers: %w", err)
  }
  ncmp := int64(0)
  if err := s.db.QueryRow(`SELECT COUNT(DISTINCT signer) FROM "completions"`).Scan(&ncmp); err != nil {
    return fmt.Errorf("Count completion signers: %w", err)
  } else if ncmp > n {
    n = ncmp
  }
  want :=  n - untilPeers
  if want <= 0 {
    return nil
  }
  // Remove trusted and untrusted peers alike:
  //    - trusted peers where their most recent message is too old
  //   - untrusted peers where their first message is too old
  // Note: record deletions cascade to completions, but records may have
  // peer completions that we want deleted as well.
  rows, err := s.db.Query(`
      SELECT 
        peer,
        IF(trust > ?,
          last_seen,
          first_seen,
        ) AS recency,
        FROM "trust" R
      GROUP BY T1.signer
      ORDER BY age ASC
      LIMIT ?
  `, trustThresh, want)
  if err != nil {
    return fmt.Errorf("gather candidate peer IDs: %w", err)
  }
  defer rows.Close()
  for rows.Next() {
    peer := ""
    recency := int64(0)
    if err := rows.Scan(&peer, &recency); err != nil {
      return fmt.Errorf("scan: %w", err)
    }
    if _, err := s.db.Exec(`DELETE FROM "records" WHERE signer=? OR approver=?`); err != nil {
      return fmt.Errorf("delete %s from records: %w", peer, err)
    }
    if _, err := s.db.Exec(`DELETE FROM "completions" WHERE signer=? OR completer=?`); err != nil {
      return fmt.Errorf("delete %s from completions: %w", peer, err)
    }
  }
  return nil
}

func (s *sqlite3) deleteOldWorkability() error {
  _, err := s.db.Exec(`
    DELETE FROM "workability" WHERE uuid IN (
      SELECT C.uuid FROM "completions" C
        LEFT JOIN "records" R
        ON (C.uuid=R.uuid AND C.signer=R.signer)
      WHERE R.approver != NULL AND R.approver=R.signer
    )
  `)
  return err
}

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

func (s *sqlite3) updateWorkability(uuid string, signer string) error {
	if newWorky, err := s.ComputeRecordWorkability(uuid); err != nil {
		return err
	} else if _, err := s.db.Exec(`
			INSERT OR REPLACE 
			INTO "workability" (uuid, origin, timestamp, workability) 
			VALUES (?, ?, ?, ?);`, uuid, signer, newWorky, time.Now().Unix()); err != nil {
		return err
	}
  return nil
}

func (s *sqlite3) ComputeWorkerTrust(peer string) (float64, error) {
  completions := int64(0)
  if err := s.db.QueryRow(`SELECT COUNT(DISTINCT uuid) 
    FROM "completions" 
    WHERE completer=? AND signer=? AND timestamp!=0;`, peer, s.id).Scan(&completions); err != nil && err != sql.ErrNoRows {
    return 0, fmt.Errorf("Count completions: %w", err)
  }
  incomplete := int64(0)
  if err := s.db.QueryRow(`SELECT COUNT(DISTINCT uuid) 
    FROM "completions" 
    WHERE completer=? AND signer=? AND timestamp=0;`, peer, s.id).Scan(&incomplete); err != nil && err != sql.ErrNoRows {
    return 0, fmt.Errorf("Count incomplete: %w", err)
  }

  hearsay := int64(0)
  if err := s.db.QueryRow(`SELECT COUNT(DISTINCT uuid) FROM "completions" WHERE timestamp!=0 AND completer=? AND signer != ?;`, peer, s.id).Scan(&hearsay); err != nil && err != sql.ErrNoRows {
    return 0, fmt.Errorf("Count hearsay: %w", err)
  }
  max_hearsay := int64(0)
  if err := s.db.QueryRow(`SELECT COUNT(DISTINCT uuid) AS n FROM "completions" WHERE timestamp!=0 AND signer != ? GROUP BY completer ORDER BY n DESC LIMIT 1;`, s.id).Scan(&max_hearsay); err != nil && err != sql.ErrNoRows {
    return 0, fmt.Errorf("Max hearsay: %w", err)
  }
  //print("hearsay ", hearsay, " max ",  max_hearsay, " cplt ", completions, " incomp ", incomplete,"\n")
  return math.Max(float64(completions) + float64(hearsay)/(float64(max_hearsay)+1) - float64(incomplete), 0), nil
}

func (s *sqlite3) ComputeRecordWorkability(uuid string) (float64, error) {
  // Sum the trust for all incomplete assertions by workers
  tww := float64(0)
  if err := s.db.QueryRow(`SELECT COALESCE(SUM(T2.worker_trust), 0) FROM "completions" T1 LEFT JOIN "trust" T2 ON T1.completer=T2.peer WHERE T1.uuid=?`, uuid).Scan(&tww); err != nil {
    return 0, fmt.Errorf("Trust-weighted workers: %w", err)
  }
  //print("tww ", tww, "\n")
  pNotWork := 4/(math.Pow(4, tww+1))
  return pNotWork, nil
}

func (s *sqlite3) SetWorkerTrust(peer string, trust float64) error {
  if _, err := s.db.Exec(`
    INSERT INTO "trust" (peer, worker_trust, timestamp, first_seen, last_seen)
    VALUES ($1, $2, $3, 0, 0)
		ON CONFLICT(peer) DO UPDATE SET worker_trust=$2, timestamp=$3
    `, peer, trust, time.Now().Unix()); err != nil {
      return fmt.Errorf("setWorkerTrust: %w", err)
  }
  return nil
}
func (s *sqlite3) SetRewardTrust(peer string, trust float64) error {
  if _, err := s.db.Exec(`
    INSERT INTO "trust" (peer, reward_trust, timestamp, first_seen, last_seen)
    VALUES ($1, $2, $3, 0, 0)
		ON CONFLICT(peer) DO UPDATE SET reward_trust=$2, timestamp=$3
    `, peer, trust, time.Now().Unix()); err != nil {
      return fmt.Errorf("setRewardTrust: %w", err)
  }
  return nil
}

func (s *sqlite3) GetWorkerTrust(peer string) (float64, error) {
  // TODO consider memoization
  t := float64(0)
  err := s.db.QueryRow(`SELECT worker_trust FROM "trust" WHERE peer=?;`, peer).Scan(&t)
  if err == sql.ErrNoRows {
    return 0, nil
  }
  return t, err
}

func (s *sqlite3) SetWorkability(uuid string, origin string, workability float64) error {
  if _, err := s.db.Exec(`
    INSERT OR REPLACE INTO workability (uuid, origin, workability, timestamp)
    VALUES (?, ?, ?, ?);
    `, uuid, origin, workability, time.Now().Unix()); err != nil {
      return fmt.Errorf("SetWorkability: %w", err)
  }
  return nil
}
"""
