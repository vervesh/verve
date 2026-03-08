ALTER TABLE task ADD COLUMN number INTEGER;
CREATE UNIQUE INDEX idx_task_repo_number ON task(repo_id, number) WHERE number IS NOT NULL;
