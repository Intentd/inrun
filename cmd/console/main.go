// Command console is the console: a web UI that shows the Catalogs,
// CRDs and custom resources of one or more running runtimes, and lets
// developers submit intents through the gateway's generated forms. It reads
// everything over the runtime and gateway HTTP APIs and has no Kubernetes
// access of its own.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	v "github.com/intentd/inrun/console/version"
	"github.com/intentd/inrun/console/web"
)

var (
	version   = v.Version
	commit    = v.Commit
	buildDate = v.Date
)

func printVersion() {
	fmt.Printf("inrun-console %s (commit %s, built %s)\n",
		version, commit, buildDate)
}

func main() {
	kfg := web.NewConsoleConfig()

	// Parse flags
	var (
		inrunURLs     = flag.String("u", strings.Join(kfg.URLs, ","), "Comma-separated URLs of Inrun runtime instances")
		ignoreDefault = flag.Bool("ignore-default", kfg.IgnoreDefault, "Do not add the default localhost:8080 URL; start with no instances")
		port          = flag.String("p", kfg.Port, "Port to serve the console on")
		refresh       = flag.Duration("refresh", kfg.RefreshInterval, "Refresh interval for fetching Catalogs")
		logLevel      = flag.String("log-level", kfg.LogLevel, "Log level (debug, info, warn, error)")
		showVersion   = flag.Bool("version", false, "Show version information")
	)

	flag.Parse()
	web.SetLogLevel(*logLevel)

	if *showVersion {
		printVersion()
		os.Exit(0)
	}

	// Parse and deduplicate URLs
	urls := parseAndDedupeURLs(*inrunURLs)
	if len(urls) == 0 && !*ignoreDefault {
		log.Fatal("ERROR: at least one Inrun URL is required. Use -u flag or --ignore-default.")
	}

	if len(urls) == 0 {
		urls = append(urls, "http://localhost:8080")
	}

	// Create console
	c := web.New(urls, web.Config{
		RefreshInterval:      *refresh,
		LogLevel:             *logLevel,
		Version:              version,
		EnableRuntimeManager: kfg.EnableRuntimeManager,
		NoLogin:              kfg.NoLogin,
		GatewayToken:         kfg.GatewayToken,
	})

	// Setup routes
	mux := setupRoutes(c, version, commit, buildDate)

	// Setup HTTP server
	srv := &http.Server{
		Addr:         ":" + *port,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		web.Infof("Inrun Console starting on :%s", *port)
		web.Infof("Watching instances: %v", urls)
		web.Infof("Refresh interval: %v", *refresh)
		web.Infof("Console URL: http://localhost:%s/console", *port)
		web.Debugf("Health endpoint: http://localhost:%s/console/health", *port)
		web.Debugf("Ready endpoint: http://localhost:%s/console/ready", *port)
		web.Debugf("Version endpoint: http://localhost:%s/console/version", *port)

		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("❌ Server error: %v", err)
		}
	}()

	<-ctx.Done()
	web.Infof("Shutting down gracefully...")

	c.Close()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		web.Warnf("Shutdown error: %v", err)
	}

	web.Infof("Console stopped")
}

// parseAndDedupeURLs parses a comma-separated URL string and removes duplicates
func parseAndDedupeURLs(input string) []string {
	parts := strings.Split(input, ",")
	seen := make(map[string]bool)
	urls := make([]string, 0, len(parts))

	for _, p := range parts {
		url := strings.TrimSpace(p)
		if url == "" {
			continue
		}

		// Add scheme if missing
		if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
			url = "http://" + url
		}

		// Remove trailing slash
		url = strings.TrimSuffix(url, "/")

		// Deduplicate
		if !seen[url] {
			seen[url] = true
			urls = append(urls, url)
		}
	}

	return urls
}
