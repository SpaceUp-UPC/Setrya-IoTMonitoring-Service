package mysql

import (
	"strings"
	"testing"
)

func TestParseDSN(t *testing.T) {
	cfg, err := parseDSN("mysql://ana:s3cr%40t@db.example.com:25060/iot?ssl-mode=REQUIRED")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.User != "ana" || cfg.Passwd != "s3cr@t" || cfg.Addr != "db.example.com:25060" || cfg.DBName != "iot" || cfg.TLSConfig != "true" || cfg.Net != "tcp" {
		t.Fatalf("got %+v", cfg)
	}
	if cfg, err := parseDSN("mysql://u:p@host/db"); err != nil || cfg.Addr != "host:3306" || cfg.TLSConfig != "" {
		t.Fatalf("default port: %+v err=%v", cfg, err)
	}
	if cfg, err := parseDSN("u:p@tcp(localhost:3306)/db"); err != nil || cfg.Addr != "localhost:3306" || cfg.DBName != "db" {
		t.Fatalf("driver format: %+v err=%v", cfg, err)
	}
	// El error no debe incluir el DSN: lleva la contraseña.
	if _, err := parseDSN("mysql://u:hunter2@ho st/db"); err == nil || strings.Contains(err.Error(), "hunter2") {
		t.Fatalf("got %v", err)
	}
}
