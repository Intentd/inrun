package external

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/intentd/inrun/pkg/types"
	"github.com/intentd/inrun/pkg/utils"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// mongoClient executes a MongoDB find query via the query: field.
//
// query: syntax: "database.collection" — runs find({}) and returns all documents.
// For filtered queries, append a JSON filter: "database.collection {\"status\":\"active\"}"
//
// The credential from auth: is injected as the password, overriding any
// password in the url:.
//
// Result map keys:
//
//	result      — JSON string of the first document (scalar case)
//	documents   — []map[string]interface{} for all matched documents
//	count       — number of documents returned as a string
//	error       — error message string, empty on success
//	called      — "true"
type mongoClient struct{}

func (c *mongoClient) Fetch(ctx context.Context, spec types.ExternalCallSpec, resolvedURL, resolvedQuery, _, credential string) (map[string]interface{}, error) {
	if resolvedQuery == "" {
		return errorResult("mongo: query: is required (e.g. \"mydb.mycollection\" or \"mydb.mycollection {\\\"status\\\":\\\"active\\\"}\")"), nil
	}

	timeout := defaultExternalTimeout
	if spec.Timeout != "" {
		if d, err := utils.ParseTimeDuration(spec.Timeout); err == nil {
			timeout = d
		}
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	dbName, collName, filter, err := parseMongoQuery(resolvedQuery)
	if err != nil {
		return errorResult(fmt.Sprintf("mongo: query: %v", err)), nil
	}

	clientOpts := options.Client().ApplyURI(resolvedURL)
	if credential != "" {
		existing := clientOpts.Auth
		if existing == nil {
			existing = &options.Credential{}
		}
		existing.Password = credential
		existing.PasswordSet = true
		clientOpts.SetAuth(*existing)
	}

	client, err := mongo.Connect(ctx, clientOpts)
	if err != nil {
		return errorResult(fmt.Sprintf("mongo: connect: %v", err)), nil
	}
	defer client.Disconnect(ctx)

	coll := client.Database(dbName).Collection(collName)
	cursor, err := coll.Find(ctx, filter)
	if err != nil {
		return errorResult(fmt.Sprintf("mongo: find: %v", err)), nil
	}
	defer cursor.Close(ctx)

	var docs []map[string]interface{}
	if err := cursor.All(ctx, &docs); err != nil {
		return errorResult(fmt.Sprintf("mongo: decode: %v", err)), nil
	}

	scalar := ""
	if len(docs) > 0 {
		if b, err := json.Marshal(docs[0]); err == nil {
			scalar = string(b)
		}
	}

	return map[string]interface{}{
		"result":    scalar,
		"documents": docs,
		"count":     fmt.Sprintf("%d", len(docs)),
		"error":     "",
		"called":    "true",
	}, nil
}

// parseMongoQuery splits "database.collection [optional JSON filter]" into parts.
func parseMongoQuery(query string) (dbName, collName string, filter bson.D, err error) {
	query = strings.TrimSpace(query)
	filterJSON := ""

	// Split off optional JSON filter after the first space
	if idx := strings.Index(query, " "); idx != -1 {
		filterJSON = strings.TrimSpace(query[idx+1:])
		query = query[:idx]
	}

	parts := strings.SplitN(query, ".", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", nil, fmt.Errorf("expected \"database.collection\", got %q", query)
	}
	dbName, collName = parts[0], parts[1]

	filter = bson.D{}
	if filterJSON != "" {
		if err := bson.UnmarshalExtJSON([]byte(filterJSON), true, &filter); err != nil {
			return "", "", nil, fmt.Errorf("invalid filter JSON %q: %w", filterJSON, err)
		}
	}
	return dbName, collName, filter, nil
}
