package application

// Commands

type IngestReadingCommand struct {
	DeviceID   string
	MetricType string
	Value      float64
	Unit       string
}

type AcknowledgeAlertCommand struct{ AlertID string }

type CloseAlertCommand struct{ AlertID string }

type RegisterDeviceCommand struct {
	SpaceID    string
	Name       string
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
