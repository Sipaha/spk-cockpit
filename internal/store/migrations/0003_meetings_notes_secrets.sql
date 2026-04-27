CREATE TABLE meetings (
  id            TEXT PRIMARY KEY,
  source        TEXT NOT NULL,
  external_uid  TEXT,
  external_etag TEXT,
  title         TEXT NOT NULL,
  description   TEXT NOT NULL DEFAULT '',
  location      TEXT NOT NULL DEFAULT '',
  start_at      INTEGER NOT NULL,
  end_at        INTEGER NOT NULL,
  notify_min    INTEGER,
  notified_at   INTEGER,
  cancelled     INTEGER NOT NULL DEFAULT 0,
  created_at    INTEGER NOT NULL,
  updated_at    INTEGER NOT NULL,
  deleted_at    INTEGER
);
CREATE UNIQUE INDEX uq_meetings_external ON meetings(source, external_uid) WHERE external_uid IS NOT NULL;
CREATE INDEX idx_meetings_start ON meetings(start_at);

CREATE TABLE notes (
  id          TEXT PRIMARY KEY,
  meeting_id  TEXT,
  todo_id     TEXT,
  body        TEXT NOT NULL DEFAULT '',
  created_at  INTEGER NOT NULL,
  updated_at  INTEGER NOT NULL,
  deleted_at  INTEGER
);
CREATE INDEX idx_notes_meeting ON notes(meeting_id) WHERE meeting_id IS NOT NULL;
CREATE INDEX idx_notes_todo    ON notes(todo_id)    WHERE todo_id    IS NOT NULL;

CREATE TABLE secrets (
  name       TEXT PRIMARY KEY,
  ciphertext BLOB NOT NULL,
  nonce      BLOB NOT NULL,
  updated_at INTEGER NOT NULL
);

CREATE TABLE sync_state (
  source     TEXT PRIMARY KEY,
  cursor     TEXT NOT NULL DEFAULT '',
  last_ok_at INTEGER,
  last_err   TEXT NOT NULL DEFAULT ''
);
