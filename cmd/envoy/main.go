package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/hotschmoe/protectorate/internal/config"
	"github.com/hotschmoe/protectorate/internal/envoy"
)

func main() {
	configPath := flag.String("config", "configs/envoy.yaml", "path to config file")
	flag.Parse()

	cfg, err := config.LoadEnvoyConfig(*configPath)
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	srv, err := envoy.NewServer(cfg)
	if err != nil {
		log.Fatalf("failed to create server: %v", err)
	}

	go func() {
		log.Printf("envoy starting on port %d", cfg.Port)
		if err := srv.Start(); err != nil {
			log.Fatalf("server error: %v", err)
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	log.Println("shutting down...")
	if err := srv.Shutdown(); err != nil {
		log.Printf("shutdown error: %v", err)
	}
}
