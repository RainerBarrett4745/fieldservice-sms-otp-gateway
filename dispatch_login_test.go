package main

import (
	"context"
	"errors"
	"testing"
)

type fakeOTP struct{ verifyErr error }

func (f *fakeOTP) SendOTP(context.Context, string, string) error   { return nil }
func (f *fakeOTP) VerifyOTP(context.Context, string, string) error { return f.verifyErr }

func TestVerifyControlsDispatchTransition(t *testing.T) {
	tests := []struct {
		name       string
		verifyErr  error
		wantStatus string
		wantErr    bool
	}{
		{name: "accepted code authenticates technician", wantStatus: "technician_authenticated"},
		{name: "rejected code does not expose work order", verifyErr: errors.New("code rejected"), wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			workflow := NewDispatchLogin(&fakeOTP{verifyErr: tt.verifyErr})
			input := WorkOrderLogin{
				WorkOrderID: "WO-2048",
				Phone:       "+15551234567",
				PhotoURLs:   []string{"https://photos.example/wo-2048/arrival.jpg"},
				FollowUp:    "Inspect compressor readings",
			}
			if err := workflow.Start(context.Background(), input, "login-WO-2048"); err != nil {
				t.Fatalf("Start() error = %v", err)
			}

			got, err := workflow.Verify(context.Background(), input.Phone, "246810")
			if (err != nil) != tt.wantErr {
				t.Fatalf("Verify() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got.Status != tt.wantStatus {
				t.Fatalf("status = %q, want %q", got.Status, tt.wantStatus)
			}
			if !tt.wantErr && (got.WorkOrderID != input.WorkOrderID || got.FollowUp != input.FollowUp || len(got.PhotoURLs) != 1) {
				t.Fatalf("verified work order was not preserved: %#v", got)
			}
		})
	}
}
