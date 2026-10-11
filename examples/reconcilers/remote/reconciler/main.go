// Remote reconciler for the WebApp CRD in Go, using only net/http. It answers
// POST /reconcile with intent-form resources; see the README for the contract.
//
//	go run main.go                              # listens on :8025
//	PORT=9000 go run main.go                    # custom port
//	RECONCILER_TOKEN=secret go run main.go      # with Bearer auth
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
)

var token = os.Getenv("RECONCILER_TOKEN")

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8025"
	}
	auth := ""
	if token != "" {
		auth = "  (auth enabled)"
	}
	log.Printf("go reconciler listening on :%s%s", port, auth)

	http.HandleFunc("/reconcile", handle)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal(err)
	}
}

// handle reads a PreparedRequest from Inrun and returns a RemoteReconcileResult.
func handle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.NotFound(w, r)
		return
	}
	if token != "" && r.Header.Get("Authorization") != "Bearer "+token {
		log.Printf("rejected: got %q", r.Header.Get("Authorization"))
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var body struct {
		Object struct {
			Metadata struct {
				Name      string `json:"name"`
				Namespace string `json:"namespace"`
			} `json:"metadata"`
			Spec struct {
				Image    string `json:"image"`
				Replicas int    `json:"replicas"`
				Port     int    `json:"port"`
			} `json:"spec"`
		} `json:"object"`
		Args struct {
			AppName     string `json:"appName"`
			LogLevel    string `json:"logLevel"`
			Environment string `json:"environment"`
		} `json:"args"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	// Prefer injected args where available; fall back to object fields.
	name := body.Args.AppName
	if name == "" {
		name = body.Object.Metadata.Name
	}
	ns := body.Object.Metadata.Namespace

	image := body.Object.Spec.Image
	if image == "" {
		image = "nginx:latest"
	}
	replicas := body.Object.Spec.Replicas
	if replicas == 0 {
		replicas = 1
	}
	port := body.Object.Spec.Port
	if port == 0 {
		port = 80
	}

	logLevel := body.Args.LogLevel
	if logLevel == "" {
		logLevel = "info"
	}
	environment := body.Args.Environment
	if environment == "" {
		environment = "development"
	}

	log.Printf("reconcile  key=%s/%s  image=%s  logLevel=%s  env=%s", ns, name, image, logLevel, environment)

	result := map[string]any{
		"result": "ok",
		"status": map[string]any{
			"phase":    "Running",
			"endpoint": fmt.Sprintf("%s-svc.%s.svc.cluster.local", name, ns),
			"replicas": replicas,
		},
		// Set on the WebApp itself; dropping a key here removes it.
		"labels":      map[string]any{"environment": environment},
		"annotations": map[string]any{"webapp.example.com/image": image},
		"resources": []any{
			map[string]any{
				"type": "deployment",
				"fields": map[string]any{
					"name":     name,
					"image":    image,
					"replicas": replicas,
					"port":     port,
				},
			},
			map[string]any{
				"type": "service",
				"fields": map[string]any{
					"name":       name + "-svc",
					"port":       80,
					"targetPort": port,
				},
			},
		},
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(result); err != nil {
		log.Printf("encode error: %v", err)
	}
}
