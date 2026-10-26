package runners

import (
	"testing"

	"k8s.io/utils/ptr"

	"github.com/intentd/inrun/pkg/types"
)

func TestResolveTarget_Target(t *testing.T) {
	dir := &types.WorkloadScaleDirection{Target: ptr.To[int32](8)}
	got := resolveTarget(2, 10, dir, true)
	if got != 8 {
		t.Fatalf("expected 8, got %d", got)
	}
}

func TestResolveTarget_TargetClampedToMax(t *testing.T) {
	dir := &types.WorkloadScaleDirection{Target: ptr.To[int32](15)}
	got := resolveTarget(2, 10, dir, true)
	if got != 10 {
		t.Fatalf("expected 10 (max), got %d", got)
	}
}

func TestResolveTarget_TargetClampedToMin(t *testing.T) {
	dir := &types.WorkloadScaleDirection{Target: ptr.To[int32](0)}
	got := resolveTarget(5, 2, dir, false)
	if got != 2 {
		t.Fatalf("expected 2 (min floor), got %d", got)
	}
}

func TestResolveTarget_Increment(t *testing.T) {
	dir := &types.WorkloadScaleDirection{Increment: ptr.To[int32](3)}
	got := resolveTarget(4, 10, dir, true)
	if got != 7 {
		t.Fatalf("expected 7, got %d", got)
	}
}

func TestResolveTarget_IncrementClampedToMax(t *testing.T) {
	dir := &types.WorkloadScaleDirection{Increment: ptr.To[int32](5)}
	got := resolveTarget(8, 10, dir, true)
	if got != 10 {
		t.Fatalf("expected 10 (max), got %d", got)
	}
}

func TestResolveTarget_Decrement(t *testing.T) {
	dir := &types.WorkloadScaleDirection{Decrement: ptr.To[int32](2)}
	got := resolveTarget(6, 2, dir, false)
	if got != 4 {
		t.Fatalf("expected 4, got %d", got)
	}
}

func TestResolveTarget_DecrementClampedToMin(t *testing.T) {
	dir := &types.WorkloadScaleDirection{Decrement: ptr.To[int32](5)}
	got := resolveTarget(3, 2, dir, false)
	if got != 2 {
		t.Fatalf("expected 2 (min floor), got %d", got)
	}
}

func TestResolveTarget_NoFields(t *testing.T) {
	dir := &types.WorkloadScaleDirection{}
	got := resolveTarget(5, 10, dir, true)
	if got != 5 {
		t.Fatalf("expected current (5) when no scaling fields set, got %d", got)
	}
}
