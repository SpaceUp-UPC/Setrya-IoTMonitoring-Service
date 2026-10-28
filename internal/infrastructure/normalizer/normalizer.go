package normalizer

import (
	"fmt"
	"math"
	"strings"

	"github.com/SpaceUp-UPC/Setrya-IoTMonitoring-Service/internal/application"
	"github.com/SpaceUp-UPC/Setrya-IoTMonitoring-Service/internal/domain"
)

// ReadingNormalizerService limpia y estandariza los valores recibidos desde
// sensores: normaliza el tipo de métrica y convierte la unidad a la canónica
// (°C, V, dB).
type ReadingNormalizerService struct{}

func New() *ReadingNormalizerService { return &ReadingNormalizerService{} }

func (ReadingNormalizerService) Normalize(cmd application.IngestReadingCommand) (application.IngestReadingCommand, error) {
	cmd.DeviceID = strings.TrimSpace(cmd.DeviceID)
	cmd.MetricType = strings.ToLower(strings.TrimSpace(cmd.MetricType))
	unit := strings.ToLower(strings.TrimSpace(cmd.Unit))

	switch domain.MetricType(cmd.MetricType) {
	case domain.MetricTemperature:
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
	case domain.MetricVoltage:
		if unit != "v" && unit != "volt" && unit != "volts" && unit != "" {
			return cmd, fmt.Errorf("%w: unsupported voltage unit %q", domain.ErrInvalid, cmd.Unit)
		}
		cmd.Unit = "V"
	case domain.MetricSound:
		if unit != "db" && unit != "" {
			return cmd, fmt.Errorf("%w: unsupported sound unit %q", domain.ErrInvalid, cmd.Unit)
		}
		cmd.Unit = "dB"
	}
	cmd.Value = math.Round(cmd.Value*100) / 100
	return cmd, nil
}
