package mysql

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"

	driver "github.com/go-sql-driver/mysql"

	"github.com/SpaceUp-UPC/Setrya-IoTMonitoring-Service/migrations"
)

// Open abre la conexión a MySQL (UTC y parseTime activados).
func Open(dsn string) (*sql.DB, error) {
	cfg, err := driver.ParseDSN(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse mysql dsn: %w", err)
	}
	cfg.ParseTime = true
	cfg.Loc = time.UTC
	db, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(20)
	db.SetConnMaxLifetime(5 * time.Minute)
	return db, nil
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
		if err != sql.ErrNoRows {
			return err
		}
		body, err := migrations.FS.ReadFile(name)
		if err != nil {
			return err
		}
		for _, stmt := range strings.Split(string(body), ";") {
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
