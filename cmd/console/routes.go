package main

import (
	"fmt"
	"net/http"

	"github.com/intentd/inrun/console/auth"
	"github.com/intentd/inrun/console/web"
)

// setupRoutes configures all HTTP routes for the Inrun Console.
func setupRoutes(c *web.Console, version, commit, buildDate string) *http.ServeMux {
	mux := http.NewServeMux()

	/* -----------------------------------------------------------
	   Public system endpoints (no auth)
	   ----------------------------------------------------------- */

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"status":"healthy","service":"inrun-console","version":"%s"}`, version)
	})

	mux.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if c.IsReady() {
			fmt.Fprintf(w, `{"status":"ready","service":"inrun-console"}`)
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
			fmt.Fprintf(w, `{"status":"not ready","service":"inrun-console","reason":"no healthy backends"}`)
		}
	})

	mux.HandleFunc("/version", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"version":"%s","commit":"%s","buildDate":"%s"}`, version, commit, buildDate)
	})

	mux.HandleFunc("/404", handleNotFound)

	/* -----------------------------------------------------------
	   Authentication UI (public)
	   ----------------------------------------------------------- */

	if c.NoLogin() {
		// NO_LOGIN mode: skip the login page, send root straight to the console.
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "/console", http.StatusFound)
		})
	} else {
		// Login page (GET)
		mux.HandleFunc("/", auth.LoginPage)

		// Login form submit (POST)
		mux.HandleFunc("/login", auth.LoginPost)

		// Logout clears session cookie
		mux.HandleFunc("/logout", auth.Logout)
	}

	/* -----------------------------------------------------------
	   Protected Console routes
	   ----------------------------------------------------------- */

	protected := http.StripPrefix("/console", c)
	if !c.NoLogin() {
		protected = auth.SessionAuth(protected)
	}
	mux.Handle("/console/", protected)

	return mux
}

// handleNotFound returns a custom 404 page
func handleNotFound(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusNotFound)
	fmt.Fprintf(w, `<!DOCTYPE html>
<html lang="en" data-theme="dark">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>404 – Inrun Console</title>
    <link rel="icon" type="image/svg+xml" href="/console/assets/static/inrun-mark.svg">
    <link rel="stylesheet" href="/console/assets/static/css/style.css">
    <script>(function(){var t=localStorage.getItem('console-theme')||'dark';document.documentElement.setAttribute('data-theme',t);})();</script>
</head>
<body style="display:flex;align-items:center;justify-content:center;min-height:100vh;background:var(--bg-base)">
    <div style="text-align:center;padding:40px;max-width:400px">
        <div style="font-size:60px;font-weight:700;color:var(--text-muted);margin-bottom:8px">404</div>
        <h1 style="font-size:18px;font-weight:600;color:var(--text-primary);margin-bottom:8px">Page not found</h1>
        <p style="font-size:13px;color:var(--text-muted);margin-bottom:6px">The page you're looking for doesn't exist or has been moved.</p>
        <a href="/console" style="display:inline-flex;align-items:center;gap:6px;padding:8px 16px;background:var(--accent);color:#fff;border-radius:6px;text-decoration:none;font-size:13px;margin-top:16px">
            ← Back to Console
        </a>
    </div>
</body>
</html>`)
}
