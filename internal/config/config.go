package config

import "os"

// Config se carga desde variables de entorno.
type Config struct {
	HTTPAddr  string
	MySQLDSN  string
	MQTTURL   string // vacío deshabilita el adaptador MQTT
	MQTTTopic string
	MQTTUser  string
	MQTTPass  string
	SMTPHost  string // host:puerto; vacío solo registra las alertas en el log
	SMTPUser  string
	SMTPPass  string
	SMTPFrom  string
	AlertTo   string
}

func Load() Config {
	return Config{
		HTTPAddr:  env("HTTP_ADDR", ":8080"),
		MySQLDSN:  env("MYSQL_DSN", "sentrya:sentrya@tcp(localhost:3306)/iot_monitoring"),
		MQTTURL:   env("MQTT_BROKER_URL", ""),
		MQTTTopic: env("MQTT_TOPIC", "sentrya/+/+/readings"),
		MQTTUser:  env("MQTT_USERNAME", ""),
		MQTTPass:  env("MQTT_PASSWORD", ""),
		SMTPHost:  env("SMTP_HOST", ""),
		SMTPUser:  env("SMTP_USERNAME", ""),
		SMTPPass:  env("SMTP_PASSWORD", ""),
		SMTPFrom:  env("SMTP_FROM", "alertas@sentrya.local"),
		AlertTo:   env("ALERT_RECIPIENT", ""),
	}
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
