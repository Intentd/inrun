package external

import (
	"context"
	"fmt"
	"strings"

	"github.com/intentd/kel/core"

	"github.com/intentd/inrun/pkg/types"
	"github.com/redis/go-redis/v9"
)

// redisClient executes a single Redis command via the query: field.
// query: syntax: "<COMMAND> [arg1] [arg2] ..."
//
//	query: "GET mykey"
//	query: "HGET myhash field"
//	query: "LLEN mylist"
//	query: "SCARD myset"
//	query: "STRLEN mykey"
//
// Result map keys:
//
//	result  — string value of the command response (always set)
//	raw     — map with "value" key containing the untyped response
//	error   — error message string, empty on success
//	called  — "true"
type redisClient struct{}

func (c *redisClient) Fetch(ctx context.Context, spec types.ExternalCallSpec, resolvedURL, resolvedQuery, _, credential string) (map[string]interface{}, error) {
	if resolvedQuery == "" {
		return errorResult("redis: query: is required (e.g. \"GET mykey\")"), nil
	}

	timeout := defaultExternalTimeout
	if spec.Timeout != "" {
		if d, err := core.ParseTimeDuration(spec.Timeout); err == nil {
			timeout = d
		}
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	opts, err := parseRedisURL(resolvedURL, credential)
	if err != nil {
		return errorResult(fmt.Sprintf("redis: invalid url: %v", err)), nil
	}

	rdb := redis.NewClient(opts)
	defer rdb.Close()

	parts := splitCommand(resolvedQuery)
	if len(parts) == 0 {
		return errorResult("redis: query: is empty"), nil
	}

	args := make([]interface{}, len(parts))
	for i, p := range parts {
		args[i] = p
	}

	cmd := rdb.Do(ctx, args...)
	val, err := cmd.Result()
	if err != nil {
		return errorResult(fmt.Sprintf("redis: %v", err)), nil
	}

	result := fmt.Sprintf("%v", val)
	return map[string]interface{}{
		"result": result,
		"raw":    map[string]interface{}{"value": val},
		"error":  "",
		"called": "true",
	}, nil
}

// parseRedisURL parses a Redis URL and injects the credential as the password.
// Accepts: redis://host:port, redis://:password@host:port, rediss://host:port
func parseRedisURL(rawURL, credential string) (*redis.Options, error) {
	opts, err := redis.ParseURL(rawURL)
	if err != nil {
		// Treat as bare host:port
		opts = &redis.Options{Addr: rawURL}
	}
	if credential != "" {
		opts.Password = credential
	}
	return opts, nil
}

// splitCommand splits a Redis command string on whitespace, respecting
// double-quoted arguments (e.g. HSET myhash field "hello world").
func splitCommand(query string) []string {
	var parts []string
	var current strings.Builder
	inQuote := false
	for _, ch := range query {
		switch {
		case ch == '"':
			inQuote = !inQuote
		case ch == ' ' && !inQuote:
			if current.Len() > 0 {
				parts = append(parts, current.String())
				current.Reset()
			}
		default:
			current.WriteRune(ch)
		}
	}
	if current.Len() > 0 {
		parts = append(parts, current.String())
	}
	return parts
}
