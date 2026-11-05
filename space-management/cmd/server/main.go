package main

import (
	"database/sql"
	"log"
	"net/http"
	"os"
	"time"

	_ "github.com/go-sql-driver/mysql"

	"github.com/SpaceUp-UPC/Setrya-IoTMonitoring-Service/space-management/internal/application"
	mysqlrepo "github.com/SpaceUp-UPC/Setrya-IoTMonitoring-Service/space-management/internal/infrastructure/mysql"
	"github.com/SpaceUp-UPC/Setrya-IoTMonitoring-Service/space-management/internal/interfaces/rest"
)

func main() {
	dsn := os.Getenv("MYSQL_DSN")
	if dsn == "" { dsn = "sentrya:sentrya@tcp(localhost:3306)/space_management?parseTime=true" }
	db, err := sql.Open("mysql", dsn)
	if err != nil { log.Fatal(err) }
	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(5 * time.Minute)
	if err := db.Ping(); err != nil { log.Fatal(err) }

	app := application.Service{Repo: mysqlrepo.Repository{DB: db}}
	mux := http.NewServeMux()
	mux.Handle("/api/v1/spaces", rest.Handler{App: app})
	mux.Handle("/api/v1/spaces/", rest.Handler{App: app})
	mux.Handle("/api/v1/owners/", rest.OwnerSpacesHandler{App: app})
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK); _, _ = w.Write([]byte("ok")) })

	address := os.Getenv("HTTP_ADDR")
	if address == "" { address = ":8081" }
	server := &http.Server{Addr: address, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	log.Printf("Space Management listening on %s", address)
	log.Fatal(server.ListenAndServe())
}
