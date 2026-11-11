package application

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/SpaceUp-UPC/Setrya-IoTMonitoring-Service/internal/domain"
)

// Handlers de la comunicación nube -> Edge (tópicos commands,
// security/validation y config del contrato del Edge).
//
// TODO(faltante): autorización por espacio. El JWT de IAM solo trae el
// usuario (claim sub); falta que Space Management exponga quién es dueño de
// cada espacio para restringir estas órdenes a su propietario.

type SendEdgeCommandCommandHandler struct {
	Publisher EdgePublisher
}

func (h *SendEdgeCommandCommandHandler) Handle(ctx context.Context, cmd SendEdgeCommandCommand) (EdgeCommandDto, error) {
	if err := domain.ValidateID("spaceId", cmd.SpaceID); err != nil {
		return EdgeCommandDto{}, err
	}
	if err := domain.ValidateID("target", cmd.Target); err != nil {
		return EdgeCommandDto{}, err
	}
	action := strings.TrimSpace(cmd.Action)
	if action == "" {
		return EdgeCommandDto{}, fmt.Errorf("%w: action is required", domain.ErrInvalid)
	}
	params := cmd.Params
	if params == nil {
		params = map[string]any{}
	}
	edgeCmd := EdgeCommand{CommandID: domain.NewID(), Action: action, Params: params}
	if err := h.Publisher.PublishCommand(ctx, cmd.SpaceID, cmd.Target, edgeCmd); err != nil {
		return EdgeCommandDto{}, err
	}
	return EdgeCommandDto{CommandID: edgeCmd.CommandID, Target: cmd.Target, Action: action}, nil
}

type PushEdgeConfigCommandHandler struct {
	Publisher EdgePublisher
}

func (h *PushEdgeConfigCommandHandler) Handle(ctx context.Context, cmd PushEdgeConfigCommand) error {
	if err := domain.ValidateID("spaceId", cmd.SpaceID); err != nil {
		return err
	}
	var tree map[string]any
	if err := json.Unmarshal(cmd.Config, &tree); err != nil || len(tree) == 0 {
		return fmt.Errorf("%w: config must be a non-empty JSON object", domain.ErrInvalid)
	}
	return h.Publisher.PublishConfig(ctx, cmd.SpaceID, cmd.Config)
}

const (
	DecisionConfirm = "confirm"
	DecisionReject  = "reject"
)

// ValidateSecurityAlertCommandHandler envía al Edge la decisión del usuario
// sobre una intrusión. Confirmar reconoce la alerta (el Edge escala a
// autoridades); rechazar la cierra (el Edge apaga la sirena).
type ValidateSecurityAlertCommandHandler struct {
	Alerts    domain.IAlertRepository
	Publisher EdgePublisher
	Now       Clock
}

func (h *ValidateSecurityAlertCommandHandler) Handle(ctx context.Context, cmd ValidateSecurityAlertCommand) (AlertDto, error) {
	if cmd.Decision != DecisionConfirm && cmd.Decision != DecisionReject {
		return AlertDto{}, fmt.Errorf("%w: decision must be %q or %q", domain.ErrInvalid, DecisionConfirm, DecisionReject)
	}
	alert, err := h.Alerts.FindByID(ctx, cmd.AlertID)
	if err != nil {
		return AlertDto{}, err
	}
	if alert.Metric != domain.MetricIntrusion || alert.EventID == "" {
		return AlertDto{}, domain.ErrNotValidatable
	}
	if !alert.IsPending() {
		return AlertDto{}, fmt.Errorf("%w: alert was already %s", domain.ErrInvalidTransition, alert.Status)
	}
	// Se publica antes de cambiar el estado: si el broker falla, el usuario puede reintentar.
	if err := h.Publisher.PublishValidation(ctx, alert.SpaceID, alert.EventID, cmd.Decision); err != nil {
		return AlertDto{}, err
	}
	if cmd.Decision == DecisionConfirm {
		err = alert.Acknowledge(h.Now())
	} else {
		err = alert.Close(h.Now())
	}
	if err != nil {
		return AlertDto{}, err
	}
	if err := h.Alerts.Save(ctx, alert); err != nil {
		return AlertDto{}, fmt.Errorf("save alert: %w", err)
	}
	return ToAlertDto(alert), nil
}
