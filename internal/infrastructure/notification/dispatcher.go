package notification

import (
	"context"
	"fmt"
	"log/slog"
	"net/smtp"
	"strings"

	"github.com/SpaceUp-UPC/Setrya-IoTMonitoring-Service/internal/domain"
)

// SMTPConfig configura el envío de correo. Con Host vacío, el dispatcher solo
// registra la alerta en el log.
type SMTPConfig struct {
	Host     string // host:puerto
	Username string
	Password string
	From     string
	// Recipient es el destinatario fijo de las alertas.
	//
	// TODO(faltante): el correo del dueño del hogar vive en IAM / Space
	// Management; falta definir cómo obtenerlo (API REST entre servicios o
	// dato incluido en el dispositivo). Mientras tanto se usa un destinatario
	// configurado por entorno.
	Recipient string
}

// NotificationDispatcherService envía las alertas al usuario responsable.
type NotificationDispatcherService struct {
	cfg SMTPConfig
	log *slog.Logger
}

func NewDispatcher(cfg SMTPConfig, log *slog.Logger) *NotificationDispatcherService {
	return &NotificationDispatcherService{cfg: cfg, log: log}
}

func (d *NotificationDispatcherService) Notify(_ context.Context, alert *domain.Alert, device *domain.IoTDevice) error {
	subject := fmt.Sprintf("[Sentrya] Alerta %s: %s", alert.Severity, device.Name)
	if d.cfg.Host == "" || d.cfg.Recipient == "" {
		d.log.Info("alert notification (smtp disabled)", "subject", subject, "message", alert.Message)
		return nil
	}

	var auth smtp.Auth
	if d.cfg.Username != "" {
		host := d.cfg.Host
		if i := strings.LastIndex(host, ":"); i >= 0 {
			host = host[:i]
		}
		auth = smtp.PlainAuth("", d.cfg.Username, d.cfg.Password, host)
	}
	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s\r\n",
		d.cfg.From, d.cfg.Recipient, subject, alert.Message)
	return smtp.SendMail(d.cfg.Host, auth, d.cfg.From, []string{d.cfg.Recipient}, []byte(msg))
}
