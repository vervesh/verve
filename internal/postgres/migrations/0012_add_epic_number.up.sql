ALTER TABLE epic ADD COLUMN number INTEGER;
CREATE UNIQUE INDEX idx_epic_repo_number ON epic(repo_id, number) WHERE number IS NOT NULL;
