package external

import (
	"context"
	"fmt"

	"github.com/intentd/inrun/pkg/types"
	"github.com/intentd/inrun/pkg/utils"
	"github.com/jackc/pgx/v5"
)

// postgresClient executes a SQL query via the query: field.
//
// query: is a SQL string. The credential from auth: is injected as the
// password, overriding any password in the url:.
//
// Result map keys:
//
//	result    — first column of the first row as a string (scalar case)
//	rows      — []map[string]interface{} for multi-row/multi-column results
//	rowCount  — number of rows returned as a string
//	error     — error message string, empty on success
//	called    — "true"
type postgresClient struct{}

func (c *postgresClient) Fetch(ctx context.Context, spec types.ExternalCallSpec, resolvedURL, resolvedQuery, _, credential string) (map[string]interface{}, error) {
	if resolvedQuery == "" {
		return errorResult("postgres: query: is required"), nil
	}

	timeout := defaultExternalTimeout
	if spec.Timeout != "" {
		if d, err := utils.ParseTimeDuration(spec.Timeout); err == nil {
			timeout = d
		}
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	connStr := resolvedURL
	if credential != "" {
		cfg, err := pgx.ParseConfig(resolvedURL)
		if err != nil {
			return errorResult(fmt.Sprintf("postgres: invalid url: %v", err)), nil
		}
		cfg.Password = credential
		connStr = cfg.ConnString()
	}

	conn, err := pgx.Connect(ctx, connStr)
	if err != nil {
		return errorResult(fmt.Sprintf("postgres: connect: %v", err)), nil
	}
	defer conn.Close(ctx)

	rows, err := conn.Query(ctx, resolvedQuery)
	if err != nil {
		return errorResult(fmt.Sprintf("postgres: query: %v", err)), nil
	}
	defer rows.Close()

	fields := rows.FieldDescriptions()
	var resultRows []map[string]interface{}
	for rows.Next() {
		vals, err := rows.Values()
		if err != nil {
			return errorResult(fmt.Sprintf("postgres: scan: %v", err)), nil
		}
		row := make(map[string]interface{}, len(fields))
		for i, f := range fields {
			row[string(f.Name)] = vals[i]
		}
		resultRows = append(resultRows, row)
	}
	if err := rows.Err(); err != nil {
		return errorResult(fmt.Sprintf("postgres: rows: %v", err)), nil
	}

	scalar := ""
	if len(resultRows) > 0 {
		for _, v := range resultRows[0] {
			scalar = fmt.Sprintf("%v", v)
			break
		}
	}

	return map[string]interface{}{
		"result":   scalar,
		"rows":     resultRows,
		"rowCount": fmt.Sprintf("%d", len(resultRows)),
		"error":    "",
		"called":   "true",
	}, nil
}
