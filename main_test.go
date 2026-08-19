package main

import (
	"strings"
	"testing"

	certmgrv1 "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
)

func TestGodaddyDNSSolverValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *godaddyDNSProviderConfig
		wantErr string
	}{
		{
			name: "missing secret name",
			cfg: &godaddyDNSProviderConfig{
				APIKeySecretRef: certmgrv1.SecretKeySelector{Key: "token"},
			},
			wantErr: "apiKeySecretRef.name must be set",
		},
		{
			name: "missing secret key",
			cfg: &godaddyDNSProviderConfig{
				APIKeySecretRef: certmgrv1.SecretKeySelector{
					LocalObjectReference: certmgrv1.LocalObjectReference{Name: "godaddy-secret"},
				},
			},
			wantErr: "apiKeySecretRef.key must be set",
		},
		{
			name: "unsupported api version",
			cfg: &godaddyDNSProviderConfig{
				APIKeySecretRef: certmgrv1.SecretKeySelector{
					LocalObjectReference: certmgrv1.LocalObjectReference{Name: "godaddy-secret"},
					Key:                  "token",
				},
				APIVersion: "v2",
			},
			wantErr: "apiVersion must be one of: v1, v3",
		},
		{
			name: "supported api version",
			cfg: &godaddyDNSProviderConfig{
				APIKeySecretRef: certmgrv1.SecretKeySelector{
					LocalObjectReference: certmgrv1.LocalObjectReference{Name: "godaddy-secret"},
					Key:                  "token",
				},
				APIVersion: "v3",
			},
		},
	}

	solver := &godaddyDNSSolver{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := solver.validate(tt.cfg)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("expected no error, got %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error %q, got nil", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error containing %q, got %q", tt.wantErr, err.Error())
			}
		})
	}
}
