package rest

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/SpaceUp-UPC/Setrya-IoTMonitoring-Service/internal/application"
	"github.com/SpaceUp-UPC/Setrya-IoTMonitoring-Service/internal/domain"
)

const maxBodyBytes = 1 << 20

// Contratos mínimos de los handlers de la capa de aplicación.
type (
	ingestHandler interface {
		Handle(context.Context, application.IngestReadingCommand) (application.IngestResult, error)
	}
	ackHandler interface {
		Handle(context.Context, application.AcknowledgeAlertCommand) (application.AlertDto, error)
	}
	closeHandler interface {
		Handle(context.Context, application.CloseAlertCommand) (application.AlertDto, error)
	}
	readingsHandler interface {
		Handle(context.Context, application.GetReadingsBySpaceQuery) ([]application.ReadingDto, error)
	}
	alertsHandler interface {
		Handle(context.Context, application.GetAlertsBySpaceQuery) ([]application.AlertDto, error)
	}
	summaryHandler interface {
		Handle(context.Context, application.GetMonitoringSummaryQuery) (application.MonitoringSummaryDto, error)
	}
	registerDeviceHandler interface {
		Handle(context.Context, application.RegisterDeviceCommand) (application.DeviceDto, error)
	}
	devicesHandler interface {
		Handle(context.Context, application.GetDevicesBySpaceQuery) ([]application.DeviceDto, error)
	}
	setActiveHandler interface {
		Activate(context.Context, application.ActivateDeviceCommand) (application.DeviceDto, error)
		Deactivate(context.Context, application.DeactivateDeviceCommand) (application.DeviceDto, error)
	}
)

// IoTMonitoringController expone la ingesta y consulta de lecturas.
type IoTMonitoringController struct {
	Ingest   ingestHandler
	Readings readingsHandler
	Summary  summaryHandler
	Log      *slog.Logger
}

// POST /api/v1/readings
func (c *IoTMonitoringController) PostReading(w http.ResponseWriter, r *http.Request) {
	var res IngestReadingResource
	if !decode(w, r, &res) {
		return
	}
	cmd, err := IngestReadingCommandFromResourceAssembler(res)
	if err != nil {
		writeError(w, c.Log, err)
		return
	}
	result, err := c.Ingest.Handle(r.Context(), cmd)
	if err != nil {
		writeError(w, c.Log, err)
		return
	}
	writeJSON(w, http.StatusCreated, IngestResultResourceAssembler(result))
}

// GET /api/v1/spaces/{spaceId}/readings?limit=
func (c *IoTMonitoringController) GetReadingsBySpace(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	dtos, err := c.Readings.Handle(r.Context(), application.GetReadingsBySpaceQuery{SpaceID: r.PathValue("spaceId"), Limit: limit})
	if err != nil {
		writeError(w, c.Log, err)
		return
	}
	out := make([]ReadingResource, 0, len(dtos))
	for _, d := range dtos {
		out = append(out, ReadingResourceFromEntityAssembler(d))
	}
	writeJSON(w, http.StatusOK, out)
}

// GET /api/v1/spaces/{spaceId}/monitoring-summary
func (c *IoTMonitoringController) GetSummary(w http.ResponseWriter, r *http.Request) {
	dto, err := c.Summary.Handle(r.Context(), application.GetMonitoringSummaryQuery{SpaceID: r.PathValue("spaceId")})
	if err != nil {
		writeError(w, c.Log, err)
		return
	}
	writeJSON(w, http.StatusOK, MonitoringSummaryResourceAssembler(dto))
}

// AlertController expone la consulta y gestión de alertas.
type AlertController struct {
	Alerts      alertsHandler
	Acknowledge ackHandler
	Close       closeHandler
	Log         *slog.Logger
}

// GET /api/v1/spaces/{spaceId}/alerts?status=
func (c *AlertController) GetAlertsBySpace(w http.ResponseWriter, r *http.Request) {
	dtos, err := c.Alerts.Handle(r.Context(), application.GetAlertsBySpaceQuery{
		SpaceID: r.PathValue("spaceId"), Status: r.URL.Query().Get("status"),
	})
	if err != nil {
		writeError(w, c.Log, err)
		return
	}
	out := make([]AlertResource, 0, len(dtos))
	for _, d := range dtos {
		out = append(out, AlertResourceFromEntityAssembler(d))
	}
	writeJSON(w, http.StatusOK, out)
}

// PATCH /api/v1/alerts/{alertId}/acknowledge
func (c *AlertController) PatchAcknowledge(w http.ResponseWriter, r *http.Request) {
	cmd := AcknowledgeAlertCommandFromResourceAssembler(AcknowledgeAlertResource{AlertID: r.PathValue("alertId")})
	dto, err := c.Acknowledge.Handle(r.Context(), cmd)
	if err != nil {
		writeError(w, c.Log, err)
		return
	}
	writeJSON(w, http.StatusOK, AlertResourceFromEntityAssembler(dto))
}

// PATCH /api/v1/alerts/{alertId}/close
func (c *AlertController) PatchClose(w http.ResponseWriter, r *http.Request) {
	cmd := CloseAlertCommandFromResourceAssembler(CloseAlertResource{AlertID: r.PathValue("alertId")})
	dto, err := c.Close.Handle(r.Context(), cmd)
	if err != nil {
		writeError(w, c.Log, err)
		return
	}
	writeJSON(w, http.StatusOK, AlertResourceFromEntityAssembler(dto))
}

// DeviceController expone el registro y activación de dispositivos. El
// informe no define estos endpoints, pero la ingesta exige un dispositivo
// registrado y activo.
type DeviceController struct {
	Register  registerDeviceHandler
	Devices   devicesHandler
	SetActive setActiveHandler
	Log       *slog.Logger
}

// POST /api/v1/devices
func (c *DeviceController) PostDevice(w http.ResponseWriter, r *http.Request) {
	var res RegisterDeviceResource
	if !decode(w, r, &res) {
		return
	}
	dto, err := c.Register.Handle(r.Context(), RegisterDeviceCommandFromResourceAssembler(res))
	if err != nil {
		writeError(w, c.Log, err)
		return
	}
	writeJSON(w, http.StatusCreated, DeviceResourceFromEntityAssembler(dto))
}

// GET /api/v1/spaces/{spaceId}/devices
func (c *DeviceController) GetDevicesBySpace(w http.ResponseWriter, r *http.Request) {
	dtos, err := c.Devices.Handle(r.Context(), application.GetDevicesBySpaceQuery{SpaceID: r.PathValue("spaceId")})
	if err != nil {
		writeError(w, c.Log, err)
		return
	}
	out := make([]DeviceResource, 0, len(dtos))
	for _, d := range dtos {
		out = append(out, DeviceResourceFromEntityAssembler(d))
	}
	writeJSON(w, http.StatusOK, out)
}

// PATCH /api/v1/devices/{deviceId}/activate
func (c *DeviceController) PatchActivate(w http.ResponseWriter, r *http.Request) {
	dto, err := c.SetActive.Activate(r.Context(), application.ActivateDeviceCommand{DeviceID: r.PathValue("deviceId")})
	if err != nil {
		writeError(w, c.Log, err)
		return
	}
	writeJSON(w, http.StatusOK, DeviceResourceFromEntityAssembler(dto))
}

// PATCH /api/v1/devices/{deviceId}/deactivate
func (c *DeviceController) PatchDeactivate(w http.ResponseWriter, r *http.Request) {
	dto, err := c.SetActive.Deactivate(r.Context(), application.DeactivateDeviceCommand{DeviceID: r.PathValue("deviceId")})
	if err != nil {
		writeError(w, c.Log, err)
		return
	}
	writeJSON(w, http.StatusOK, DeviceResourceFromEntityAssembler(dto))
}

func decode(w http.ResponseWriter, r *http.Request, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResource{Error: "invalid JSON body"})
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, log *slog.Logger, err error) {
	switch {
	case errors.Is(err, domain.ErrInvalid):
		writeJSON(w, http.StatusBadRequest, ErrorResource{Error: err.Error()})
	case errors.Is(err, domain.ErrDeviceNotFound), errors.Is(err, domain.ErrAlertNotFound):
		writeJSON(w, http.StatusNotFound, ErrorResource{Error: err.Error()})
	case errors.Is(err, domain.ErrDeviceInactive), errors.Is(err, domain.ErrInvalidTransition):
		writeJSON(w, http.StatusConflict, ErrorResource{Error: err.Error()})
	default:
		log.Error("request failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, ErrorResource{Error: "internal error"})
	}
}
