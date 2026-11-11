package config

import (
	"errors"
	"os"
	"strings"
)

// Config se carga desde variables de entorno.
type Config struct {
	HTTPAddr string
	MySQLDSN string
	MySQLTLS string // "", "true", "skip-verify" o "preferred"

	MQTTURL      string // vacío deshabilita el adaptador MQTT
	MQTTTopic    string
	MQTTClientID string
	MQTTUser     string
	MQTTPass     string

	SMTPHost string // host:puerto; vacío solo registra las alertas en el log
	SMTPUser string
	SMTPPass string
	SMTPFrom string
	AlertTo  string

	JWTSecret    string // el mismo authorization.jwt.secret de IAM
	AuthDisabled bool   // solo para desarrollo local
	CORSOrigins  []string
}

func Load() (Config, error) {
	cfg := Config{
		HTTPAddr:     httpAddr(),
		MySQLDSN:     env("MYSQL_DSN", "sentrya:sentrya@tcp(localhost:3306)/iot_monitoring"),
		MySQLTLS:     env("MYSQL_TLS", ""),
		MQTTURL:      env("MQTT_BROKER_URL", ""),
		MQTTTopic:    env("MQTT_TOPIC", "sentrya/+/+/readings"),
		MQTTClientID: env("MQTT_CLIENT_ID", "sentrya-iot-monitoring"),
		MQTTUser:     env("MQTT_USERNAME", ""),
		MQTTPass:     env("MQTT_PASSWORD", ""),
		SMTPHost:     env("SMTP_HOST", ""),
		SMTPUser:     env("SMTP_USERNAME", ""),
		SMTPPass:     env("SMTP_PASSWORD", ""),
		SMTPFrom:     env("SMTP_FROM", "alertas@sentrya.local"),
		AlertTo:      env("ALERT_RECIPIENT", ""),
		JWTSecret:    env("JWT_SECRET", ""),
		AuthDisabled: env("AUTH_DISABLED", "") == "true",
		CORSOrigins:  splitList(env("CORS_ALLOWED_ORIGINS", "*")),
	}
	// Sin secreto la API quedaría abierta, incluidas las órdenes a
	// actuadores: hay que pedirlo de forma explícita.
	if cfg.JWTSecret == "" && !cfg.AuthDisabled {
		return cfg, errors.New("JWT_SECRET is required (or set AUTH_DISABLED=true for local development)")
	}
	return cfg, nil
}

// httpAddr respeta PORT, la variable con la que plataformas como Render
// indican el puerto a escuchar.
func httpAddr() string {
	if addr := os.Getenv("HTTP_ADDR"); addr != "" {
		return addr
	}
	if port := os.Getenv("PORT"); port != "" {
		return ":" + port
	}
	return ":8080"
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func splitList(s string) []string {
	var out []string
	for item := range strings.SplitSeq(s, ",") {
		if item = strings.TrimSpace(item); item != "" {
			out = append(out, item)
		}
	}
	return out
}
