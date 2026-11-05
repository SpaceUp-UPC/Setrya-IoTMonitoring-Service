CREATE TABLE IF NOT EXISTS spaces (
 id VARCHAR(64) PRIMARY KEY, owner_id VARCHAR(64) NOT NULL, name VARCHAR(160) NOT NULL,
 description TEXT NOT NULL, location_json JSON NOT NULL, dimensions_json JSON NOT NULL,
 status VARCHAR(24) NOT NULL, created_at DATETIME(6) NOT NULL, updated_at DATETIME(6) NOT NULL,
 INDEX idx_spaces_owner_created (owner_id, created_at)
);
