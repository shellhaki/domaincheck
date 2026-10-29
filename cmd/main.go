package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"shellhaki/domaincheck/config"
	"shellhaki/domaincheck/internals"

	"github.com/joho/godotenv"
)

type JSON map[string]any

func writeJSON(w http.ResponseWriter, code int, data JSON) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(data)
}

func main() {
	_ = godotenv.Load()
	config.Load()

	client := &http.Client{Timeout: 10 * time.Second}
	registry := internals.NewRegistry(client, config.Vars.RDAPBootstrapURL)
	bootstrapCtx, cancelBootstrap := context.WithTimeout(context.Background(), 15*time.Second)
	err := registry.Load(bootstrapCtx)
	cancelBootstrap()
	if err != nil {
		log.Fatalf("load RDAP bootstrap: %v", err)
	}

	rootCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go registry.RefreshLoop(rootCtx)
	cache := internals.NewResultCache(60*time.Second, 10000)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, JSON{"message": "api running"})
	})
	mux.HandleFunc("GET /domain/{domain}", func(w http.ResponseWriter, r *http.Request) {
		domain, err := internals.NormalizeDomain(r.PathValue("domain"))
		if err != nil {
			writeJSON(w, http.StatusBadRequest, JSON{"error": err.Error()})
			return
		}
		if result, ok := cache.Get(domain); ok {
			writeJSON(w, http.StatusOK, JSON{"data": result})
			return
		}
		result, err := internals.Lookup(r.Context(), domain, registry)
		if err != nil {
			log.Printf("domain lookup %q: %v", domain, err)
			writeJSON(w, http.StatusBadGateway, JSON{"error": "domain lookup could not be completed", "data": result})
			return
		}
		cache.Set(domain, result)
		writeJSON(w, http.StatusOK, JSON{"data": result})
	})
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, JSON{"status": "ok"})
	})

	server := &http.Server{Addr: ":" + config.Vars.Port, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		<-rootCtx.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(ctx)
	}()
	log.Printf("domain checker listening on %s", server.Addr)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}
