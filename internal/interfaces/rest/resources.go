package rest

import "time"

// Resources de entrada

type IngestReadingResource struct {
	DeviceID   string   `json:"deviceId"`
	MetricType string   `json:"metricType"`
	Value      *float64 `json:"value"`
	Unit       string   `json:"unit"`

	// Campos opcionales, los mismos del payload MQTT del Edge.
	ReadingID   string     `json:"readingId"`
	MeasuredAt  *time.Time `json:"measuredAt"`
	LocalState  string     `json:"localState"`
	Reason      string     `json:"reason"`
	EventID     string     `json:"eventId"`
	EvidenceRef string     `json:"evidenceRef"`
}

// AcknowledgeAlertResource lleva el id de la alerta a reconocer (tomado de la ruta).
type AcknowledgeAlertResource struct {
	AlertID string `json:"alertId"`
}

// CloseAlertResource lleva el id de la alerta a cerrar (tomado de la ruta).
type CloseAlertResource struct {
	AlertID string `json:"alertId"`
}

type ThresholdResource struct {
	MetricType string  `json:"metricType"`
	MinValue   float64 `json:"minValue"`
	MaxValue   float64 `json:"maxValue"`
	Unit       string  `json:"unit"`
}

type RegisterDeviceResource struct {
	// ID es opcional: un id acordado con el Edge (ej. "space1-temperature").
	ID         string              `json:"id"`
	SpaceID    string              `json:"spaceId"`
	Name       string              `json:"name"`
	Active     bool                `json:"active"`
	Thresholds []ThresholdResource `json:"thresholds"`
}

// ValidateAlertResource lleva la decisión del usuario sobre una intrusión.
type ValidateAlertResource struct {
	Decision string `json:"decision"`
}

// EdgeCommandResource es una orden para un actuador del Edge.
type EdgeCommandResource struct {
	Action string         `json:"action"`
	Params map[string]any `json:"params"`
}

// Resources de salida

type ReadingResource struct {
	ID         string     `json:"id"`
	DeviceID   string     `json:"deviceId"`
	MetricType string     `json:"metricType"`
	Value      float64    `json:"value"`
	Unit       string     `json:"unit"`
	ReceivedAt time.Time  `json:"receivedAt"`
	Status     string     `json:"status"`
	MeasuredAt *time.Time `json:"measuredAt,omitempty"`
	LocalState string     `json:"localState,omitempty"`
}

type AlertResource struct {
	ID             string     `json:"id"`
	ReadingID      string     `json:"readingId"`
	DeviceID       string     `json:"deviceId"`
	SpaceID        string     `json:"spaceId"`
	MetricType     string     `json:"metricType"`
	Message        string     `json:"message"`
	Severity       string     `json:"severity"`
	Status         string     `json:"status"`
	CreatedAt      time.Time  `json:"createdAt"`
	AcknowledgedAt *time.Time `json:"acknowledgedAt,omitempty"`
	ClosedAt       *time.Time `json:"closedAt,omitempty"`
	Reason         string     `json:"reason,omitempty"`
	EventID        string     `json:"eventId,omitempty"`
	EvidenceRef    string     `json:"evidenceRef,omitempty"`
}

type IngestResultResource struct {
	Reading ReadingResource `json:"reading"`
	Alert   *AlertResource  `json:"alert,omitempty"`
	// Duplicate es true si la lectura ya se había recibido (mismo readingId).
	Duplicate bool `json:"duplicate,omitempty"`
}

type EdgeCommandAcceptedResource struct {
	CommandID string `json:"commandId"`
	Target    string `json:"target"`
	Action    string `json:"action"`
}

type DeviceResource struct {
	ID         string              `json:"id"`
	SpaceID    string              `json:"spaceId"`
	Name       string              `json:"name"`
	Active     bool                `json:"active"`
	Thresholds []ThresholdResource `json:"thresholds"`
	CreatedAt  time.Time           `json:"createdAt"`
}

type MonitoringSummaryResource struct {
	SpaceID        string `json:"spaceId"`
	RecentReadings int    `json:"recentReadings"`
	ActiveAlerts   int    `json:"activeAlerts"`
	Status         string `json:"status"`
}

type ErrorResource struct {
	Error string `json:"error"`
}
