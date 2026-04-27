CREATE TABLE timer_sessions (
  id         INTEGER PRIMARY KEY AUTOINCREMENT,
  todo_id    TEXT NOT NULL,
  started_at INTEGER NOT NULL,
  ended_at   INTEGER,
  source     TEXT NOT NULL DEFAULT 'manual'
);
CREATE INDEX idx_timer_todo ON timer_sessions(todo_id, started_at);
CREATE UNIQUE INDEX uq_timer_active ON timer_sessions(todo_id) WHERE ended_at IS NULL;
