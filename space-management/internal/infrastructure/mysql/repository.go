package mysql

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/SpaceUp-UPC/Setrya-IoTMonitoring-Service/space-management/internal/application"
	"github.com/SpaceUp-UPC/Setrya-IoTMonitoring-Service/space-management/internal/domain"
)

type Repository struct {
	DB *sql.DB
}

func (r Repository) Save(ctx context.Context, space domain.Space) error {
	location, _ := json.Marshal(space.Location)
	dimensions, _ := json.Marshal(space.Dimensions)

	_, err := r.DB.ExecContext(ctx, `
		INSERT INTO spaces (
			id, owner_id, name, description, location_json,
			dimensions_json, status, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			name = VALUES(name), description = VALUES(description),
			location_json = VALUES(location_json), dimensions_json = VALUES(dimensions_json),
			status = VALUES(status), updated_at = VALUES(updated_at)
	`, space.ID, space.OwnerID, space.Name, space.Description,
		location, dimensions, space.Status, space.CreatedAt, space.UpdatedAt)
	return err
}

func (r Repository) Find(ctx context.Context, id string) (domain.Space, error) {
	var space domain.Space
	var location, dimensions []byte

	err := r.DB.QueryRowContext(ctx, `
		SELECT id, owner_id, name, description, location_json,
			dimensions_json, status, created_at, updated_at
		FROM spaces WHERE id = ?
	`, id).Scan(
		&space.ID, &space.OwnerID, &space.Name, &space.Description,
		&location, &dimensions, &space.Status, &space.CreatedAt, &space.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return space, application.ErrNotFound
	}
	if err != nil {
		return space, err
	}

	_ = json.Unmarshal(location, &space.Location)
	_ = json.Unmarshal(dimensions, &space.Dimensions)
	return space, nil
}

func (r Repository) ByOwner(ctx context.Context, ownerID string) ([]domain.Space, error) {
	rows, err := r.DB.QueryContext(ctx, `
		SELECT id, owner_id, name, description, location_json,
			dimensions_json, status, created_at, updated_at
		FROM spaces WHERE owner_id = ? ORDER BY created_at DESC
	`, ownerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	spaces := []domain.Space{}
	for rows.Next() {
		var space domain.Space
		var location, dimensions []byte
		if err := rows.Scan(
			&space.ID, &space.OwnerID, &space.Name, &space.Description,
			&location, &dimensions, &space.Status, &space.CreatedAt, &space.UpdatedAt,
		); err != nil {
			return nil, err
		}

		_ = json.Unmarshal(location, &space.Location)
		_ = json.Unmarshal(dimensions, &space.Dimensions)
		spaces = append(spaces, space)
	}
	return spaces, rows.Err()
}
