CREATE TABLE todos (
  id            TEXT PRIMARY KEY,
  title         TEXT NOT NULL,
  notes         TEXT NOT NULL DEFAULT '',
  priority      INTEGER NOT NULL,
  status        TEXT NOT NULL,
  due_at        INTEGER,
  created_at    INTEGER NOT NULL,
  updated_at    INTEGER NOT NULL,
  done_at       INTEGER,
  deleted_at    INTEGER
);
CREATE INDEX idx_todos_status_priority ON todos(status, priority DESC, due_at);
CREATE INDEX idx_todos_done_at ON todos(done_at) WHERE done_at IS NOT NULL;

CREATE TABLE tags (
  name       TEXT PRIMARY KEY,
  color      TEXT NOT NULL DEFAULT '',
  created_at INTEGER NOT NULL
);

CREATE TABLE todo_tags (
  todo_id TEXT NOT NULL REFERENCES todos(id) ON DELETE CASCADE,
  tag     TEXT NOT NULL REFERENCES tags(name) ON DELETE CASCADE ON UPDATE CASCADE,
  PRIMARY KEY (todo_id, tag)
);
CREATE INDEX idx_todo_tags_tag ON todo_tags(tag);

CREATE TABLE todo_events (
  id         INTEGER PRIMARY KEY AUTOINCREMENT,
  todo_id    TEXT NOT NULL,
  kind       TEXT NOT NULL,
  from_value TEXT,
  to_value   TEXT,
  payload    TEXT,
  at         INTEGER NOT NULL
);
CREATE INDEX idx_todo_events_todo_at ON todo_events(todo_id, at);

CREATE TABLE kv (
  k TEXT PRIMARY KEY,
  v TEXT NOT NULL
);
