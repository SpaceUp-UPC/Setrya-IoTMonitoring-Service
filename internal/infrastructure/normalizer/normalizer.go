package normalizer

import (
	"fmt"
	"math"
	"strings"

	"github.com/SpaceUp-UPC/Setrya-IoTMonitoring-Service/internal/application"
	"github.com/SpaceUp-UPC/Setrya-IoTMonitoring-Service/internal/domain"
)

// ReadingNormalizerService limpia y estandariza los valores recibidos desde
// sensores: normaliza el tipo de métrica, convierte la unidad a la canónica
// (°C, %, V, dB, event) y valida el estado local que envía el Edge.
type ReadingNormalizerService struct{}

func New() *ReadingNormalizerService { return &ReadingNormalizerService{} }

func (ReadingNormalizerService) Normalize(cmd application.IngestReadingCommand) (application.IngestReadingCommand, error) {
	cmd.DeviceID = strings.TrimSpace(cmd.DeviceID)
	cmd.ReadingID = strings.TrimSpace(cmd.ReadingID)
	cmd.MetricType = strings.ToLower(strings.TrimSpace(cmd.MetricType))
	cmd.Reason = clip(cmd.Reason, 64)
	cmd.EventID = clip(cmd.EventID, 64)
	cmd.EvidenceRef = clip(cmd.EvidenceRef, 255)
	unit := strings.ToLower(strings.TrimSpace(cmd.Unit))

	switch state := domain.LocalState(strings.ToUpper(strings.TrimSpace(cmd.LocalState))); state {
	case domain.LocalStateNone, domain.LocalStateNormal, domain.LocalStateWarning, domain.LocalStateCritical:
		cmd.LocalState = string(state)
	default:
		// Un estado desconocido no invalida la lectura; simplemente no se usa.
		cmd.LocalState = ""
	}

	metric := domain.MetricType(cmd.MetricType)
	switch {
	case metric == domain.MetricTemperature:
		switch unit {
		case "c", "°c", "celsius", "":
			cmd.Unit = "°C"
		case "f", "°f", "fahrenheit":
			cmd.Value = (cmd.Value - 32) * 5 / 9
			cmd.Unit = "°C"
		case "k", "kelvin":
			cmd.Value -= 273.15
			cmd.Unit = "°C"
		default:
			return cmd, fmt.Errorf("%w: unsupported temperature unit %q", domain.ErrInvalid, cmd.Unit)
		}
	case metric == domain.MetricHumidity:
		if unit != "%" && unit != "%rh" && unit != "percent" && unit != "" {
			return cmd, fmt.Errorf("%w: unsupported humidity unit %q", domain.ErrInvalid, cmd.Unit)
		}
		cmd.Unit = "%"
	case metric == domain.MetricVoltage:
		if unit != "v" && unit != "volt" && unit != "volts" && unit != "" {
			return cmd, fmt.Errorf("%w: unsupported voltage unit %q", domain.ErrInvalid, cmd.Unit)
		}
		cmd.Unit = "V"
	case metric == domain.MetricSound:
		if unit != "db" && unit != "" {
			return cmd, fmt.Errorf("%w: unsupported sound unit %q", domain.ErrInvalid, cmd.Unit)
		}
		cmd.Unit = "dB"
	case metric.IsEvent():
		if unit != domain.EventUnit && unit != "" {
			return cmd, fmt.Errorf("%w: unsupported event unit %q", domain.ErrInvalid, cmd.Unit)
		}
		cmd.Unit = domain.EventUnit
	}
	cmd.Value = math.Round(cmd.Value*100) / 100
	return cmd, nil
}

// clip recorta un texto libre que llega de fuera al largo de su columna.
func clip(s string, max int) string {
	s = strings.TrimSpace(s)
	if r := []rune(s); len(r) > max {
		return string(r[:max])
	}
	return s
}
