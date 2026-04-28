-- Per-todo "hide from Done column without waiting for the 3-day cutoff"
-- flag. Set by POST /api/todos/{id}/dismiss; cleared automatically when the
-- status moves away from done.
ALTER TABLE todos ADD COLUMN dismissed_at INTEGER;
