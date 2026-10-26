package web

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type ConsoleConfig struct {
	Port                 string        `json:"port"`
	IgnoreDefault        bool          `json:"ignoreDefault"`
	RefreshInterval      time.Duration `json:"refreshInterval"`
	LogLevel             string        `json:"logLevel"`
	URLs                 []string      `json:"urls"`
	EnableRuntimeManager bool          `json:"enableRuntimeManager"`
	NoLogin              bool          `json:"noLogin"`
	GatewayToken         string        `json:"gatewayToken"`
}

func NewConsoleConfig() *ConsoleConfig {
	return handleEnvVars()
}

// Handle user defined variables
func handleEnvVars() *ConsoleConfig {
	port := getStrEnv("PORT", "8081")
	ignoreDefault := getBoolEnv("IGNORE_DEFAULT", false)
	runtimeManager := getBoolEnv("ENABLE_RUNTIME_MANAGER", true)
	noLogin := getBoolEnv("NO_LOGIN", false)
	loglevel := getStrEnv("LOG_LEVEL", "info")
	refreshInterval := getDurEnv("REFRESH_INTERVAL", 15)
	urls := splitEnv("INRUN_URLS", []string{})
	gatewayToken := getStrEnv("GATEWAY_TOKEN", "")

	// PUBLIC_DEPLOYMENT=true is a shorthand that implies NO_LOGIN=true and
	// ENABLE_RUNTIME_MANAGER=false. Individual vars can still override it.
	if getBoolEnv("PUBLIC_DEPLOYMENT", false) {
		noLogin = true
		runtimeManager = false
	}

	return &ConsoleConfig{
		Port:                 port,
		IgnoreDefault:        ignoreDefault,
		RefreshInterval:      refreshInterval,
		LogLevel:             loglevel,
		URLs:                 urls,
		EnableRuntimeManager: runtimeManager,
		NoLogin:              noLogin,
		GatewayToken:         gatewayToken,
	}
}

func getStrEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func splitEnv(key string, defaultValue []string) []string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}

	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))

	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		result = append(result, p)
	}

	return result
}

func splitEnvUpper(key string, defaultValue []string) []string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}

	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))

	for _, p := range parts {
		p = strings.TrimSpace(strings.ToUpper(p))
		if p == "" {
			continue
		}
		result = append(result, p)
	}

	return result
}

func getBoolEnv(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if valueBool, err := strconv.ParseBool(value); err == nil {
			return valueBool
		}
	}
	return defaultValue
}

func getIntEnv(key string, defaultValue int) int64 {
	if value := os.Getenv(key); value != "" {
		if valueInt, err := strconv.ParseInt(value, 10, 64); err == nil {
			return valueInt
		}
	}
	return int64(defaultValue)
}

// Duration in seconds
func getDurEnv(key string, defaultValue int) time.Duration {
	if value := os.Getenv(key); value != "" {
		if valueInt, err := strconv.Atoi(value); err == nil {
			return time.Duration(valueInt) * time.Second
		}
	}

	return time.Duration(defaultValue) * time.Second
}
