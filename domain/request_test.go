package domain

import "testing"

func TestRequest_Finalizing(t *testing.T) {
	if (Request{}).Finalizing() {
		t.Error("no prepared state: want false")
	}
	if !(Request{Prepared: &PreparedRequest{Finalizing: true}}).Finalizing() {
		t.Error("prepared as finalizing: want true")
	}
}
