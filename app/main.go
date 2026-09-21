package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/lestrrat-go/server-starter/listener"
)

var (
	// Version will be set at build time by GoReleaser (-ldflags "-X main.Version={{.Version}}")
	Version = "dev"
)

func main() {
	versionFlag := flag.Bool("version", false, "Print version and exit")
	portFlag := flag.String("port", "8080", "Port to bind to when not running under server-starter")
	flag.Parse()

	if *versionFlag {
		fmt.Printf("Dewy App Version: %s\n", Version)
		os.Exit(0)
	}

	log.Printf("Starting Dewy App (Version: %s)...", Version)

	// Attempt to retrieve listeners from server-starter (Dewy binary mode)
	listeners, err := listener.ListenAll()
	if err != nil && err != listener.ErrNoListeningTarget {
		log.Fatalf("Failed to initialize server-starter listener: %v", err)
	}

	var l net.Listener
	if len(listeners) > 0 {
		log.Printf("Running under server-starter. Inheriting listening socket.")
		l = listeners[0]
	} else {
		// Fallback: Listen directly (local development or container mode)
		addr := ":" + *portFlag
		log.Printf("No server-starter listener detected. Falling back to bind direct on %s", addr)
		l, err = net.Listen("tcp", addr)
		if err != nil {
			log.Fatalf("Failed to listen on %s: %v", addr, err)
		}
	}

	// Setup handlers
	mux := http.NewServeMux()
	mux.HandleFunc("/", handleHome)
	mux.HandleFunc("/health", handleHealth)

	server := &http.Server{
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	// Start server in background
	go func() {
		log.Printf("Server listening on %s", l.Addr().String())
		if err := server.Serve(l); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	// Graceful shutdown handling
	// server-starter sends SIGTERM to stop the old process during rolling restart
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	sig := <-stop
	log.Printf("Received signal: %v. Initiating graceful shutdown...", sig)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Graceful shutdown failed: %v", err)
	}

	log.Println("Dewy App stopped gracefully.")
}

func handleHome(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	hostname, _ := os.Hostname()
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = fmt.Fprintf(w, "Hello from Dewy App!\nVersion: %s\nHostname: %s\nTime: %s\n", Version, hostname, time.Now().Format(time.RFC3339))
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}
