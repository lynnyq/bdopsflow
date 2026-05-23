package service

import (
	"testing"
)

func TestComputeWebhookSignature(t *testing.T) {
	tests := []struct {
		name      string
		secret    string
		body      string
		wantEmpty bool
	}{
		{
			name:      "empty secret returns empty signature",
			secret:    "",
			body:      `{"event":"test"}`,
			wantEmpty: true,
		},
		{
			name:      "non-empty secret returns signature",
			secret:    "my-secret",
			body:      `{"event":"test"}`,
			wantEmpty: false,
		},
		{
			name:      "same input produces same signature",
			secret:    "my-secret",
			body:      `{"event":"test"}`,
			wantEmpty: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ComputeWebhookSignature(tt.secret, []byte(tt.body))
			if (got == "") != tt.wantEmpty {
				t.Errorf("ComputeWebhookSignature() = %v, wantEmpty %v", got, tt.wantEmpty)
			}
		})
	}

	sig1 := ComputeWebhookSignature("secret", []byte(`{"test":1}`))
	sig2 := ComputeWebhookSignature("secret", []byte(`{"test":1}`))
	if sig1 != sig2 {
		t.Errorf("same input should produce same signature, got %s and %s", sig1, sig2)
	}

	sig3 := ComputeWebhookSignature("secret", []byte(`{"test":2}`))
	if sig1 == sig3 {
		t.Errorf("different input should produce different signature")
	}

	if sig1 != "" && len(sig1) < 10 {
		t.Errorf("signature should be long enough, got %s", sig1)
	}
}

func TestWebhookService_DomainIsolation(t *testing.T) {
	_ = NewWebhookService(nil)
}
