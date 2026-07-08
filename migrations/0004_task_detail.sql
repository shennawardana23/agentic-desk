-- Task Management: long-form detail body for the task detail modal.
-- `notes` (0002_task.sql) stays the board card's short 2-line preview;
-- `description` is the modal's full-length body — distinct fields, same
-- NOT NULL DEFAULT '' convention as notes so Task.Description stays a
-- plain string, no *string/sql.NullString needed.
ALTER TABLE task ADD COLUMN IF NOT EXISTS description TEXT NOT NULL DEFAULT '';
