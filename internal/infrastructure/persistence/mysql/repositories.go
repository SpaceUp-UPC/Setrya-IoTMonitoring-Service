package mysql

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/SpaceUp-UPC/Setrya-IoTMonitoring-Service/internal/domain"
)

type thresholdJSON struct {
	Min  float64 `json:"min"`
	Max  float64 `json:"max"`
	Unit string  `json:"unit"`
}

// IoTDeviceRepository implementa domain.IIoTDeviceRepository.
type IoTDeviceRepository struct{ db *sql.DB }

func NewIoTDeviceRepository(db *sql.DB) *IoTDeviceRepository { return &IoTDeviceRepository{db: db} }

func (r *IoTDeviceRepository) Save(ctx context.Context, d *domain.IoTDevice) error {
	ths := make(map[string]thresholdJSON, len(d.Thresholds))
	for m, t := range d.Thresholds {
		ths[string(m)] = thresholdJSON{Min: t.MinValue, Max: t.MaxValue, Unit: t.Unit}
	}
	raw, err := json.Marshal(ths)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, `
		INSERT INTO iot_devices (id, space_id, name, active, thresholds, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE name = VALUES(name), active = VALUES(active), thresholds = VALUES(thresholds)`,
		d.ID, d.SpaceID, d.Name, d.Active, raw, d.CreatedAt.UTC())
	return err
}

const deviceColumns = `id, space_id, name, active, thresholds, created_at`

type scanner interface{ Scan(dest ...any) error }

func scanDevice(s scanner) (*domain.IoTDevice, error) {
	var d domain.IoTDevice
	var raw []byte
	if err := s.Scan(&d.ID, &d.SpaceID, &d.Name, &d.Active, &raw, &d.CreatedAt); err != nil {
		return nil, err
	}
	var ths map[string]thresholdJSON
	if err := json.Unmarshal(raw, &ths); err != nil {
		return nil, err
	}
	d.Thresholds = make(map[domain.MetricType]domain.ThresholdRange, len(ths))
	for m, t := range ths {
		d.Thresholds[domain.MetricType(m)] = domain.ThresholdRange{MinValue: t.Min, MaxValue: t.Max, Unit: t.Unit}
	}
	return &d, nil
}

func (r *IoTDeviceRepository) FindByID(ctx context.Context, id string) (*domain.IoTDevice, error) {
	d, err := scanDevice(r.db.QueryRowContext(ctx, `SELECT `+deviceColumns+` FROM iot_devices WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrDeviceNotFound
	}
	return d, err
}

func (r *IoTDeviceRepository) FindBySpace(ctx context.Context, spaceID string) ([]*domain.IoTDevice, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT `+deviceColumns+` FROM iot_devices WHERE space_id = ? ORDER BY created_at`, spaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*domain.IoTDevice
	for rows.Next() {
		d, err := scanDevice(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// ReadingRepository implementa domain.IReadingRepository.
type ReadingRepository struct{ db *sql.DB }

func NewReadingRepository(db *sql.DB) *ReadingRepository { return &ReadingRepository{db: db} }

func (r *ReadingRepository) Save(ctx context.Context, rd *domain.Reading) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO iot_readings (id, device_id, metric_type, value, unit, status, received_at, measured_at, local_state)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE status = VALUES(status)`,
		rd.ID, rd.DeviceID, string(rd.Metric), rd.Value, rd.Unit, string(rd.Status), rd.ReceivedAt.UTC(),
		nullTime(rd.MeasuredAt), string(rd.LocalState))
	return err
}

const readingColumns = `r.id, r.device_id, r.metric_type, r.value, r.unit, r.received_at, r.status, r.measured_at, r.local_state`

func scanReading(s scanner) (*domain.Reading, error) {
	var rd domain.Reading
	var metric, status, localState string
	var measured sql.NullTime
	if err := s.Scan(&rd.ID, &rd.DeviceID, &metric, &rd.Value, &rd.Unit, &rd.ReceivedAt, &status, &measured, &localState); err != nil {
		return nil, err
	}
	rd.Metric, rd.Status, rd.LocalState = domain.MetricType(metric), domain.ReadingStatus(status), domain.LocalState(localState)
	if measured.Valid {
		rd.MeasuredAt = &measured.Time
	}
	return &rd, nil
}

func (r *ReadingRepository) FindByID(ctx context.Context, id string) (*domain.Reading, error) {
	rd, err := scanReading(r.db.QueryRowContext(ctx, `SELECT `+readingColumns+` FROM iot_readings r WHERE r.id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrReadingNotFound
	}
	return rd, err
}

func (r *ReadingRepository) FindBySpace(ctx context.Context, spaceID string, limit int) ([]*domain.Reading, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT `+readingColumns+`
		FROM iot_readings r JOIN iot_devices d ON d.id = r.device_id
		WHERE d.space_id = ? ORDER BY r.received_at DESC LIMIT ?`, spaceID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*domain.Reading
	for rows.Next() {
		rd, err := scanReading(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, rd)
	}
	return out, rows.Err()
}

// AlertRepository implementa domain.IAlertRepository.
type AlertRepository struct{ db *sql.DB }

func NewAlertRepository(db *sql.DB) *AlertRepository { return &AlertRepository{db: db} }

func nullTime(t *time.Time) sql.NullTime {
	if t == nil {
		return sql.NullTime{}
	}
	return sql.NullTime{Time: t.UTC(), Valid: true}
}

func (r *AlertRepository) Save(ctx context.Context, a *domain.Alert) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO iot_alerts (id, reading_id, device_id, space_id, metric_type, severity, status, message,
			created_at, acknowledged_at, closed_at, reason, event_id, evidence_ref)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE status = VALUES(status), acknowledged_at = VALUES(acknowledged_at), closed_at = VALUES(closed_at)`,
		a.ID, a.ReadingID, a.DeviceID, a.SpaceID, string(a.Metric), string(a.Severity), string(a.Status), a.Message,
		a.CreatedAt.UTC(), nullTime(a.AcknowledgedAt), nullTime(a.ClosedAt), a.Reason, a.EventID, a.EvidenceRef)
	return err
}

const alertColumns = `id, reading_id, device_id, space_id, metric_type, severity, status, message,
	created_at, acknowledged_at, closed_at, reason, event_id, evidence_ref`

func scanAlert(s scanner) (*domain.Alert, error) {
	var a domain.Alert
	var metric, severity, status string
	var ack, closed sql.NullTime
	if err := s.Scan(&a.ID, &a.ReadingID, &a.DeviceID, &a.SpaceID, &metric, &severity, &status, &a.Message,
		&a.CreatedAt, &ack, &closed, &a.Reason, &a.EventID, &a.EvidenceRef); err != nil {
		return nil, err
	}
	a.Metric, a.Severity, a.Status = domain.MetricType(metric), domain.SeverityLevel(severity), domain.AlertStatus(status)
	if ack.Valid {
		a.AcknowledgedAt = &ack.Time
	}
	if closed.Valid {
		a.ClosedAt = &closed.Time
	}
	return &a, nil
}

func (r *AlertRepository) FindByID(ctx context.Context, id string) (*domain.Alert, error) {
	a, err := scanAlert(r.db.QueryRowContext(ctx, `SELECT `+alertColumns+` FROM iot_alerts WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrAlertNotFound
	}
	return a, err
}

func (r *AlertRepository) FindByReading(ctx context.Context, readingID string) (*domain.Alert, error) {
	a, err := scanAlert(r.db.QueryRowContext(ctx, `SELECT `+alertColumns+` FROM iot_alerts WHERE reading_id = ? LIMIT 1`, readingID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return a, err
}

func (r *AlertRepository) FindBySpace(ctx context.Context, spaceID string, status *domain.AlertStatus) ([]*domain.Alert, error) {
	query := `SELECT ` + alertColumns + ` FROM iot_alerts WHERE space_id = ?`
	args := []any{spaceID}
	if status != nil {
		query += ` AND status = ?`
		args = append(args, string(*status))
	}
	return r.query(ctx, query+` ORDER BY created_at DESC`, args...)
}

func (r *AlertRepository) FindOpenByDevice(ctx context.Context, deviceID string, metric domain.MetricType) ([]*domain.Alert, error) {
	return r.query(ctx, `SELECT `+alertColumns+` FROM iot_alerts
		WHERE device_id = ? AND metric_type = ? AND status <> ? ORDER BY created_at DESC`,
		deviceID, string(metric), string(domain.AlertClosed))
}

func (r *AlertRepository) query(ctx context.Context, query string, args ...any) ([]*domain.Alert, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*domain.Alert
	for rows.Next() {
		a, err := scanAlert(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}
