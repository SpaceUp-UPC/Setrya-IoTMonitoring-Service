package rest

import (
	"net/http"

	"github.com/SpaceUp-UPC/Setrya-IoTMonitoring-Service/api"
)

// NewRouter registra las rutas del servicio sobre el ServeMux estándar.
//
// TODO(faltante): autenticación. El informe no define cómo se valida el
// acceso (JWT emitido por IAM validado en el API Gateway o en cada servicio)
// ni cómo se autentican los dispositivos que publican lecturas por HTTP.
// Cuando exista, se agrega aquí como middleware.
func NewRouter(m *IoTMonitoringController, a *AlertController, d *DeviceController) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /openapi.yaml", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/yaml")
		_, _ = w.Write(api.OpenAPI)
	})

	mux.HandleFunc("POST /api/v1/readings", m.PostReading)
	mux.HandleFunc("GET /api/v1/spaces/{spaceId}/readings", m.GetReadingsBySpace)
	mux.HandleFunc("GET /api/v1/spaces/{spaceId}/monitoring-summary", m.GetSummary)

	mux.HandleFunc("GET /api/v1/spaces/{spaceId}/alerts", a.GetAlertsBySpace)
	mux.HandleFunc("PATCH /api/v1/alerts/{alertId}/acknowledge", a.PatchAcknowledge)
	mux.HandleFunc("PATCH /api/v1/alerts/{alertId}/close", a.PatchClose)

	mux.HandleFunc("POST /api/v1/devices", d.PostDevice)
	mux.HandleFunc("GET /api/v1/spaces/{spaceId}/devices", d.GetDevicesBySpace)
	mux.HandleFunc("PATCH /api/v1/devices/{deviceId}/activate", d.PatchActivate)
	mux.HandleFunc("PATCH /api/v1/devices/{deviceId}/deactivate", d.PatchDeactivate)

	return mux
}
