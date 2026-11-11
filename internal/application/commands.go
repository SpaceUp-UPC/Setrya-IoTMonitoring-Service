package application

import (
	"encoding/json"
	"time"
)

// Commands

type IngestReadingCommand struct {
	DeviceID   string
	MetricType string
	Value      float64
	Unit       string

	// Campos opcionales del contrato del Edge.
	ReadingID   string
	MeasuredAt  *time.Time
	LocalState  string
	Reason      string
	EventID     string
	EvidenceRef string
}

type AcknowledgeAlertCommand struct{ AlertID string }

type CloseAlertCommand struct{ AlertID string }

// ValidateSecurityAlertCommand lleva la decisión del usuario sobre una
// intrusión: "confirm" o "reject".
type ValidateSecurityAlertCommand struct {
	AlertID  string
	Decision string
}

type RegisterDeviceCommand struct {
	// ID es opcional: permite ids acordados con el Edge.
	ID         string
	SpaceID    string
	Name       string
	Active     bool
	Thresholds []ThresholdInput
}

type ThresholdInput struct {
	MetricType string
	MinValue   float64
	MaxValue   float64
	Unit       string
}

type ActivateDeviceCommand struct{ DeviceID string }

type DeactivateDeviceCommand struct{ DeviceID string }

// SendEdgeCommandCommand ordena una acción sobre un actuador del Edge.
// Target es el deviceId registrado o el nombre del actuador (valve, lock,
// humidifier).
type SendEdgeCommandCommand struct {
	SpaceID string
	Target  string
	Action  string
	Params  map[string]any
}

// PushEdgeConfigCommand envía umbrales al Edge; Config es el árbol de
// umbrales tal como lo espera su perfil.
type PushEdgeConfigCommand struct {
	SpaceID string
	Config  json.RawMessage
}

// Queries

type GetReadingsBySpaceQuery struct {
	SpaceID string
	Limit   int
}

type GetAlertsBySpaceQuery struct {
	SpaceID string
	// Status vacío no filtra.
	Status string
}

type GetDevicesBySpaceQuery struct{ SpaceID string }

type GetMonitoringSummaryQuery struct{ SpaceID string }
