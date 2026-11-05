package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/SpaceUp-UPC/Setrya-IoTMonitoring-Service/internal/application"
	"github.com/SpaceUp-UPC/Setrya-IoTMonitoring-Service/internal/config"
	"github.com/SpaceUp-UPC/Setrya-IoTMonitoring-Service/internal/domain"
	"github.com/SpaceUp-UPC/Setrya-IoTMonitoring-Service/internal/infrastructure/mqtt"
	"github.com/SpaceUp-UPC/Setrya-IoTMonitoring-Service/internal/infrastructure/normalizer"
	"github.com/SpaceUp-UPC/Setrya-IoTMonitoring-Service/internal/infrastructure/notification"
	mysqlrepo "github.com/SpaceUp-UPC/Setrya-IoTMonitoring-Service/internal/infrastructure/persistence/mysql"
	"github.com/SpaceUp-UPC/Setrya-IoTMonitoring-Service/internal/interfaces/rest"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg := config.Load()

	db, err := mysqlrepo.Open(cfg.MySQLDSN)
	if err != nil {
		fatal(log, "open database", err)
	}
	defer db.Close()
	if err := waitForDB(db.PingContext, log); err != nil {
		fatal(log, "database unreachable", err)
	}
	if err := mysqlrepo.Migrate(db); err != nil {
		fatal(log, "migrate database", err)
	}

	devices := mysqlrepo.NewIoTDeviceRepository(db)
	readings := mysqlrepo.NewReadingRepository(db)
	alerts := mysqlrepo.NewAlertRepository(db)

	policy := domain.NewAlertPolicyService()
	monitoring := domain.NewMonitoringDomainService(policy)
	dispatcher := notification.NewDispatcher(notification.SMTPConfig{
		Host: cfg.SMTPHost, Username: cfg.SMTPUser, Password: cfg.SMTPPass, From: cfg.SMTPFrom, Recipient: cfg.AlertTo,
	}, log)
	now := application.Clock(func() time.Time { return time.Now().UTC() })

	ingest := &application.IngestReadingCommandHandler{
		Devices: devices, Readings: readings, Alerts: alerts, Monitoring: monitoring, Policy: policy,
		Notifier: dispatcher, Normalizer: normalizer.New(), Now: now, Log: log,
	}

	router := rest.NewRouter(
		&rest.IoTMonitoringController{
			Ingest:   ingest,
			Readings: &application.GetReadingsBySpaceQueryHandler{Readings: readings},
			Summary:  &application.GetMonitoringSummaryQueryHandler{Readings: readings, Alerts: alerts},
			Log:      log,
		},
		&rest.AlertController{
			Alerts:      &application.GetAlertsBySpaceQueryHandler{Alerts: alerts},
			Acknowledge: &application.AcknowledgeAlertCommandHandler{Alerts: alerts, Now: now},
			Close:       &application.CloseAlertCommandHandler{Alerts: alerts, Now: now},
			Log:         log,
		},
		&rest.DeviceController{
			Register:  &application.RegisterDeviceCommandHandler{Devices: devices, Now: now},
			Devices:   &application.GetDevicesBySpaceQueryHandler{Devices: devices},
			SetActive: &application.SetDeviceActiveCommandHandler{Devices: devices},
			Log:       log,
		},
	)

	if cfg.MQTTURL != "" {
		adapter := mqtt.NewAdapter(mqtt.Config{
			BrokerURL: cfg.MQTTURL, ClientID: "sentrya-iot-monitoring", Username: cfg.MQTTUser,
			Password: cfg.MQTTPass, Topic: cfg.MQTTTopic,
		}, ingest, log)
		if err := adapter.Start(); err != nil {
			fatal(log, "start mqtt adapter", err)
		}
		defer adapter.Stop()
	} else {
		log.Warn("MQTT_BROKER_URL not set: MQTT ingestion disabled")
	}

	srv := &http.Server{Addr: cfg.HTTPAddr, Handler: router, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		log.Info("http server listening", "addr", cfg.HTTPAddr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			fatal(log, "http server", err)
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("http shutdown", "error", err)
	}
}

// waitForDB reintenta el ping para tolerar que MySQL arranque después (docker compose).
func waitForDB(ping func(context.Context) error, log *slog.Logger) error {
	var err error
	for i := 0; i < 30; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		err = ping(ctx)
		cancel()
		if err == nil {
			return nil
		}
		log.Warn("waiting for database", "error", err)
		time.Sleep(2 * time.Second)
	}
	return err
}

func fatal(log *slog.Logger, msg string, err error) {
	log.Error(msg, "error", err)
	os.Exit(1)
}
