package rest

import "time"

// Resources de entrada

type IngestReadingResource struct {
	DeviceID   string   `json:"deviceId"`
	MetricType string   `json:"metricType"`
	Value      *float64 `json:"value"`
	Unit       string   `json:"unit"`
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
	SpaceID    string              `json:"spaceId"`
	Name       string              `json:"name"`
	Thresholds []ThresholdResource `json:"thresholds"`
}

// Resources de salida

type ReadingResource struct {
	ID         string    `json:"id"`
	DeviceID   string    `json:"deviceId"`
	MetricType string    `json:"metricType"`
	Value      float64   `json:"value"`
	Unit       string    `json:"unit"`
	ReceivedAt time.Time `json:"receivedAt"`
	Status     string    `json:"status"`
}

type AlertResource struct {
	ID             string     `json:"id"`
	ReadingID      string     `json:"readingId"`
	DeviceID       string     `json:"deviceId"`
	SpaceID        string     `json:"spaceId"`
	Message        string     `json:"message"`
	Severity       string     `json:"severity"`
	Status         string     `json:"status"`
	CreatedAt      time.Time  `json:"createdAt"`
	AcknowledgedAt *time.Time `json:"acknowledgedAt,omitempty"`
	ClosedAt       *time.Time `json:"closedAt,omitempty"`
}

type IngestResultResource struct {
	Reading ReadingResource `json:"reading"`
	Alert   *AlertResource  `json:"alert,omitempty"`
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
