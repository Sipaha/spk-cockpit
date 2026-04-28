-- Per-status manual ordering for the kanban board. REAL so we can drop a card
-- between neighbors with order = (above + below) / 2 without integer reshuffles.
ALTER TABLE todos ADD COLUMN sort_order REAL NOT NULL DEFAULT 0;

-- Seed existing rows by created_at so the initial board ordering matches the
-- old "newest first" list view.
UPDATE todos SET sort_order = CAST(created_at AS REAL);

CREATE INDEX idx_todos_sort_order ON todos(status, sort_order DESC);
