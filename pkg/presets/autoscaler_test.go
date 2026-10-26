package presets_test

import (
	"testing"
	"time"

	"github.com/intentd/inrun/pkg/presets"
	"github.com/intentd/inrun/pkg/types"
)

func baseline(workers, queue int, resync time.Duration) types.AutoscaleBaseline {
	return types.AutoscaleBaseline{
		Workers:  workers,
		MaxDepth: queue,
		Resync:   resync,
	}
}

func TestAutoscalerPresets(t *testing.T) {
	tests := []struct {
		name           string
		preset         string
		baseline       types.AutoscaleBaseline
		expectErr      bool
		expectWorkers  int
		expectQueue    int
		expectField    string
		expectInterval time.Duration
		expectCooldown time.Duration
	}{
		{
			name:           "burst",
			preset:         "burst",
			baseline:       baseline(4, 100, 120*time.Second),
			expectWorkers:  16,
			expectQueue:    1000,
			expectField:    "metrics.queueDepth",
			expectInterval: 5 * time.Second,
			expectCooldown: 30 * time.Second,
		},
		{
			name:           "steady",
			preset:         "steady",
			baseline:       baseline(4, 100, 120*time.Second),
			expectWorkers:  8,
			expectQueue:    300,
			expectField:    "metrics.queueDepth",
			expectInterval: 30 * time.Second,
			expectCooldown: 2 * time.Minute,
		},
		{
			name:           "batch",
			preset:         "batch",
			baseline:       baseline(4, 100, 120*time.Second),
			expectWorkers:  12,
			expectQueue:    800,
			expectField:    "",
			expectInterval: 60 * time.Second,
			expectCooldown: 5 * time.Minute,
		},
		{
			name:           "latency-sensitive",
			preset:         "latency-sensitive",
			baseline:       baseline(4, 100, 120*time.Second),
			expectWorkers:  10,
			expectQueue:    0,
			expectField:    "metrics.reconcileDurationP95Ms",
			expectInterval: 15 * time.Second,
			expectCooldown: 1 * time.Minute,
		},
		{
			name:           "cost-optimized",
			preset:         "cost-optimized",
			baseline:       baseline(4, 100, 120*time.Second),
			expectWorkers:  2,
			expectQueue:    50,
			expectField:    "metrics.workersIdlePercent",
			expectInterval: 30 * time.Second,
			expectCooldown: 10 * time.Minute,
		},
		{
			name:      "unknown preset fails fast",
			preset:    "unknown",
			baseline:  baseline(4, 100, 120*time.Second),
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec, err := presets.ApplyAutoscalerPreset(tt.preset, tt.baseline)

			if tt.expectErr {
				if err == nil {
					t.Fatal("expected error but got none")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if spec.Interval.Duration != tt.expectInterval {
				t.Errorf("interval: want %v got %v", tt.expectInterval, spec.Interval.Duration)
			}
			if spec.Cooldown.Duration != tt.expectCooldown {
				t.Errorf("cooldown: want %v got %v", tt.expectCooldown, spec.Cooldown.Duration)
			}
			if tt.expectWorkers > 0 {
				if spec.Do.Workers == nil || *spec.Do.Workers != tt.expectWorkers {
					t.Errorf("workers: want %d got %v", tt.expectWorkers, spec.Do.Workers)
				}
			}
			if tt.expectQueue > 0 {
				if spec.Do.QueueDepth == nil || *spec.Do.QueueDepth != tt.expectQueue {
					t.Errorf("queueDepth: want %d got %v", tt.expectQueue, spec.Do.QueueDepth)
				}
			}
			if tt.expectField != "" {
				if len(spec.Conditions.When) == 0 {
					t.Fatal("expected When conditions but got none")
				}
				if spec.Conditions.When[0].Field != tt.expectField {
					t.Errorf("condition field: want %s got %s", tt.expectField, spec.Conditions.When[0].Field)
				}
			}
		})
	}
}
