package auth

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestExtractFromSecret_Success(t *testing.T) {
	client := fake.NewSimpleClientset(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "godaddy-api-key",
			Namespace: "cert-manager",
		},
		Data: map[string][]byte{
			"token": []byte("myApiKey:myApiSecret"),
		},
	})

	creds, err := ExtractFromSecret(client, "cert-manager", "godaddy-api-key", "token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if creds.APIKey != "myApiKey" {
		t.Errorf("expected APIKey 'myApiKey', got %q", creds.APIKey)
	}
	if creds.APISecret != "myApiSecret" {
		t.Errorf("expected APISecret 'myApiSecret', got %q", creds.APISecret)
	}
}

func TestExtractFromSecret_ColonInSecret(t *testing.T) {
	client := fake.NewSimpleClientset(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "godaddy-api-key",
			Namespace: "cert-manager",
		},
		Data: map[string][]byte{
			"token": []byte("myApiKey:secret:with:colons"),
		},
	})

	creds, err := ExtractFromSecret(client, "cert-manager", "godaddy-api-key", "token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if creds.APIKey != "myApiKey" {
		t.Errorf("expected APIKey 'myApiKey', got %q", creds.APIKey)
	}
	if creds.APISecret != "secret:with:colons" {
		t.Errorf("expected APISecret 'secret:with:colons', got %q", creds.APISecret)
	}
}

func TestExtractFromSecret_SecretNotFound(t *testing.T) {
	client := fake.NewSimpleClientset()

	_, err := ExtractFromSecret(client, "cert-manager", "nonexistent", "token")
	if err == nil {
		t.Fatal("expected error for missing secret")
	}
}

func TestExtractFromSecret_KeyNotFound(t *testing.T) {
	client := fake.NewSimpleClientset(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "godaddy-api-key",
			Namespace: "cert-manager",
		},
		Data: map[string][]byte{
			"other-key": []byte("value"),
		},
	})

	_, err := ExtractFromSecret(client, "cert-manager", "godaddy-api-key", "token")
	if err == nil {
		t.Fatal("expected error for missing key")
	}
}

func TestExtractFromSecret_InvalidFormat(t *testing.T) {
	client := fake.NewSimpleClientset(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "godaddy-api-key",
			Namespace: "cert-manager",
		},
		Data: map[string][]byte{
			"token": []byte("no-colon-separator"),
		},
	})

	_, err := ExtractFromSecret(client, "cert-manager", "godaddy-api-key", "token")
	if err == nil {
		t.Fatal("expected error for invalid format")
	}
}

func TestExtractPATFromSecret_Success(t *testing.T) {
	client := fake.NewSimpleClientset(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "godaddy-pat",
			Namespace: "cert-manager",
		},
		Data: map[string][]byte{
			"token": []byte("my-personal-access-token"),
		},
	})

	pat, err := ExtractPATFromSecret(client, "cert-manager", "godaddy-pat", "token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pat != "my-personal-access-token" {
		t.Errorf("expected 'my-personal-access-token', got %q", pat)
	}
}

func TestExtractPATFromSecret_TrimWhitespace(t *testing.T) {
	client := fake.NewSimpleClientset(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "godaddy-pat",
			Namespace: "cert-manager",
		},
		Data: map[string][]byte{
			"token": []byte("  my-token\n"),
		},
	})

	pat, err := ExtractPATFromSecret(client, "cert-manager", "godaddy-pat", "token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pat != "my-token" {
		t.Errorf("expected 'my-token', got %q", pat)
	}
}

func TestExtractPATFromSecret_SecretNotFound(t *testing.T) {
	client := fake.NewSimpleClientset()

	_, err := ExtractPATFromSecret(client, "cert-manager", "nonexistent", "token")
	if err == nil {
		t.Fatal("expected error for missing secret")
	}
}

func TestExtractPATFromSecret_EmptyToken(t *testing.T) {
	client := fake.NewSimpleClientset(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "godaddy-pat",
			Namespace: "cert-manager",
		},
		Data: map[string][]byte{
			"token": []byte(""),
		},
	})

	_, err := ExtractPATFromSecret(client, "cert-manager", "godaddy-pat", "token")
	if err == nil {
		t.Fatal("expected error for empty token")
	}
}
