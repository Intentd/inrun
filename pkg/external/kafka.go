package external

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/intentd/kel/core"

	"github.com/intentd/inrun/pkg/types"
	"github.com/segmentio/kafka-go"
	"github.com/segmentio/kafka-go/sasl/plain"
)

// kafkaClient reads consumer group lag or topic metadata via the query: field.
//
// query: syntax:
//
//	"group/topic"   — consumer group lag for a topic across all partitions
//	"@topic"        — topic metadata: partition count and latest offsets
//
// auth: credential format: "username:password" (SASL/PLAIN)
//
// Result map keys (group/topic):
//
//	result      — total lag across all partitions as a string
//	lag         — total lag (same as result)
//	partitions  — number of partitions as a string
//	error       — error message string, empty on success
//	called      — "true"
//
// Result map keys (@topic):
//
//	result      — partition count as a string
//	partitions  — partition count as a string
//	error       — error message string, empty on success
//	called      — "true"
type kafkaClient struct{}

func (c *kafkaClient) Fetch(ctx context.Context, spec types.ExternalCallSpec, resolvedURL, resolvedQuery, _, credential string) (map[string]interface{}, error) {
	if resolvedQuery == "" {
		return errorResult("kafka: query: is required (\"group/topic\" or \"@topic\")"), nil
	}

	timeout := defaultExternalTimeout
	if spec.Timeout != "" {
		if d, err := core.ParseTimeDuration(spec.Timeout); err == nil {
			timeout = d
		}
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	brokers := parseBrokers(resolvedURL)
	dialer := kafkaDialer(credential)

	if strings.HasPrefix(resolvedQuery, "@") {
		return fetchTopicMeta(ctx, dialer, brokers, resolvedQuery[1:])
	}
	return fetchGroupLag(ctx, dialer, brokers, resolvedQuery)
}

// kafkaDialer returns a dialer with SASL/PLAIN when credential is "username:password",
// or the default dialer when credential is empty.
func kafkaDialer(credential string) *kafka.Dialer {
	if credential == "" {
		return kafka.DefaultDialer
	}
	username, password, _ := strings.Cut(credential, ":")
	return &kafka.Dialer{
		Timeout:       10 * time.Second,
		DualStack:     true,
		SASLMechanism: plain.Mechanism{Username: username, Password: password},
	}
}

// fetchGroupLag reads consumer group lag for "group/topic".
// Lag = sum over partitions of (latestOffset - committedOffset).
// When the group has never committed, committed is treated as 0.
func fetchGroupLag(ctx context.Context, dialer *kafka.Dialer, brokers []string, query string) (map[string]interface{}, error) {
	parts := strings.SplitN(query, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return errorResult(fmt.Sprintf("kafka: query %q: expected \"group/topic\"", query)), nil
	}
	group, topic := parts[0], parts[1]

	conn, err := dialer.DialContext(ctx, "tcp", brokers[0])
	if err != nil {
		return errorResult(fmt.Sprintf("kafka: connect: %v", err)), nil
	}
	partitions, err := conn.ReadPartitions(topic)
	conn.Close()
	if err != nil {
		return errorResult(fmt.Sprintf("kafka: read partitions for %q: %v", topic, err)), nil
	}

	partitionIDs := make([]int, len(partitions))
	for i, p := range partitions {
		partitionIDs[i] = p.ID
	}

	client := &kafka.Client{
		Addr: kafka.TCP(brokers[0]),
		Transport: &kafka.Transport{
			Dial: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return dialer.DialContext(ctx, network, addr)
			},
		},
	}

	// Committed offsets for this consumer group.
	fetchResp, err := client.OffsetFetch(ctx, &kafka.OffsetFetchRequest{
		GroupID: group,
		Topics:  map[string][]int{topic: partitionIDs},
	})
	if err != nil {
		return errorResult(fmt.Sprintf("kafka: fetch committed offsets group=%q: %v", group, err)), nil
	}

	// Latest (high-watermark) offsets.
	offsetReqs := make([]kafka.OffsetRequest, len(partitions))
	for i, p := range partitions {
		offsetReqs[i] = kafka.OffsetRequest{Partition: p.ID, Timestamp: kafka.LastOffset}
	}
	listResp, err := client.ListOffsets(ctx, &kafka.ListOffsetsRequest{
		Topics: map[string][]kafka.OffsetRequest{topic: offsetReqs},
	})
	if err != nil {
		return errorResult(fmt.Sprintf("kafka: list offsets topic=%q: %v", topic, err)), nil
	}

	committed := make(map[int]int64, len(partitions))
	for _, p := range fetchResp.Topics[topic] {
		committed[p.Partition] = p.CommittedOffset
	}
	latest := make(map[int]int64, len(partitions))
	for _, p := range listResp.Topics[topic] {
		latest[p.Partition] = p.LastOffset
	}

	var totalLag int64
	for _, id := range partitionIDs {
		c := committed[id] // 0 if group never committed
		if c < 0 {
			c = 0
		}
		l := latest[id]
		if lag := l - c; lag > 0 {
			totalLag += lag
		}
	}

	lag := fmt.Sprintf("%d", totalLag)
	return map[string]interface{}{
		"result":     lag,
		"lag":        lag,
		"partitions": fmt.Sprintf("%d", len(partitions)),
		"error":      "",
		"called":     "true",
	}, nil
}

// fetchTopicMeta reads partition count and latest offsets for "@topic".
func fetchTopicMeta(ctx context.Context, dialer *kafka.Dialer, brokers []string, topic string) (map[string]interface{}, error) {
	conn, err := dialer.DialContext(ctx, "tcp", brokers[0])
	if err != nil {
		return errorResult(fmt.Sprintf("kafka: connect: %v", err)), nil
	}
	defer conn.Close()

	partitions, err := conn.ReadPartitions(topic)
	if err != nil {
		return errorResult(fmt.Sprintf("kafka: read partitions for %q: %v", topic, err)), nil
	}

	count := fmt.Sprintf("%d", len(partitions))
	return map[string]interface{}{
		"result":         count,
		"partitionCount": count,
		"error":          "",
		"called":         "true",
	}, nil
}

// parseBrokers splits a broker list URL into individual addresses.
// Accepts: "broker1:9092,broker2:9092" or "kafka://broker:9092"
func parseBrokers(url string) []string {
	url = strings.TrimPrefix(url, "kafka://")
	var brokers []string
	for _, b := range strings.Split(url, ",") {
		if b = strings.TrimSpace(b); b != "" {
			brokers = append(brokers, b)
		}
	}
	if len(brokers) == 0 {
		brokers = []string{"localhost:9092"}
	}
	return brokers
}
