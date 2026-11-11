package mysql

import (
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"

	driver "github.com/go-sql-driver/mysql"

	"github.com/SpaceUp-UPC/Setrya-IoTMonitoring-Service/migrations"
)

// Open abre la conexión a MySQL (UTC y parseTime activados).
//
// dsn admite el formato del driver ("user:pass@tcp(host:3306)/db") o una URL
// "mysql://user:pass@host:3306/db", que es como la entregan los proveedores
// gestionados. tlsMode fija el TLS: "" (lo que indique el dsn), "true",
// "skip-verify" (proveedores con CA propia) o "preferred".
func Open(dsn, tlsMode string) (*sql.DB, error) {
	cfg, err := parseDSN(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse mysql dsn: %w", err)
	}
	cfg.ParseTime = true
	cfg.Loc = time.UTC
	if tlsMode != "" {
		cfg.TLSConfig = tlsMode
	}
	db, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(10)
	db.SetConnMaxLifetime(5 * time.Minute)
	return db, nil
}

func parseDSN(dsn string) (*driver.Config, error) {
	if !strings.HasPrefix(dsn, "mysql://") {
		return driver.ParseDSN(dsn)
	}
	u, err := url.Parse(dsn)
	if err != nil {
		// El error de url.Parse incluye el dsn completo, con la contraseña.
		return nil, errors.New("invalid mysql:// url")
	}
	cfg := driver.NewConfig()
	cfg.Net = "tcp"
	cfg.Addr = u.Host
	if u.Port() == "" {
		cfg.Addr = u.Hostname() + ":3306"
	}
	cfg.User = u.User.Username()
	cfg.Passwd, _ = u.User.Password()
	cfg.DBName = strings.TrimPrefix(u.Path, "/")
	switch strings.ToUpper(u.Query().Get("ssl-mode")) {
	case "REQUIRED", "VERIFY_CA", "VERIFY_IDENTITY":
		cfg.TLSConfig = "true"
	}
	return cfg, nil
}

// Migrate aplica en orden las migraciones embebidas que aún no se ejecutaron.
func Migrate(db *sql.DB) error {
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		name VARCHAR(100) NOT NULL PRIMARY KEY, applied_at DATETIME(6) NOT NULL)`); err != nil {
		return err
	}
	entries, err := migrations.FS.ReadDir(".")
	if err != nil {
		return err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	sort.Strings(names)

	for _, name := range names {
		var applied string
		err := db.QueryRow(`SELECT name FROM schema_migrations WHERE name = ?`, name).Scan(&applied)
		if err == nil {
			continue
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		body, err := migrations.FS.ReadFile(name)
		if err != nil {
			return err
		}
		for stmt := range strings.SplitSeq(string(body), ";") {
			if strings.TrimSpace(stmt) == "" {
				continue
			}
			if _, err := db.Exec(stmt); err != nil {
				return fmt.Errorf("migration %s: %w", name, err)
			}
		}
		if _, err := db.Exec(`INSERT INTO schema_migrations (name, applied_at) VALUES (?, UTC_TIMESTAMP(6))`, name); err != nil {
			return err
		}
	}
	return nil
}
