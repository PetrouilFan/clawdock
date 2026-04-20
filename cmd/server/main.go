package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"clawdock/internal/config"
	"clawdock/internal/database"
	"clawdock/internal/docker"
	"clawdock/internal/handlers"
	"clawdock/internal/reconciler"
)

var version = "dev"

func main() {
	configPath := flag.String("config", "/etc/openclaw-manager/config.yaml", "Path to config file")
	showVersion := flag.Bool("version", false, "Show version")
	flag.Parse()

	if *showVersion {
		fmt.Printf("openclaw-manager version %s\n", version)
		os.Exit(0)
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	db, err := database.Init(cfg.Database.Path)
	if err != nil {
		log.Fatalf("failed to init database: %v", err)
	}
	defer db.Close()

	dockerClient, err := docker.NewClient()
	if err != nil {
		log.Fatalf("failed to connect to docker: %v", err)
	}

	rec := reconciler.New(db, dockerClient)
	go rec.Run()

	h := handlers.New(cfg, db, dockerClient)

	mux := h.SetupRoutes()

	addr := cfg.Server.Host + ":" + cfg.Server.Port
	log.Printf("openclaw-manager %s starting on %s", version, addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
