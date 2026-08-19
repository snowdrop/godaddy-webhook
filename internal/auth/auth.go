package auth

import (
	"context"
	"fmt"
	"strings"

	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type Credentials struct {
	APIKey    string
	APISecret string
}

// ExtractFromSecret reads a Kubernetes Secret and parses the "key:secret" value
// into API credentials. The secret value is expected to be in the format "apiKey:apiSecret".
func ExtractFromSecret(client kubernetes.Interface, namespace, secretName, secretKey string) (*Credentials, error) {
	sec, err := client.CoreV1().
		Secrets(namespace).
		Get(context.TODO(), secretName, metaV1.GetOptions{})
	if err != nil {
		return nil, err
	}

	secBytes, ok := sec.Data[secretKey]
	if !ok {
		return nil, fmt.Errorf("key %q not found in secret %q", secretKey, fmt.Sprintf("%s/%s", namespace, secretName))
	}

	parts := strings.SplitN(string(secBytes), ":", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("secret \"%s/%s\" key %q: expected format \"apiKey:apiSecret\"", secretName, namespace, secretKey)
	}

	return &Credentials{
		APIKey:    parts[0],
		APISecret: parts[1],
	}, nil
}

// ExtractPATFromSecret reads a Kubernetes Secret and returns the raw value as a
// Personal Access Token. The secret value is used as-is (no splitting).
func ExtractPATFromSecret(client kubernetes.Interface, namespace, secretName, secretKey string) (string, error) {
	sec, err := client.CoreV1().
		Secrets(namespace).
		Get(context.TODO(), secretName, metaV1.GetOptions{})
	if err != nil {
		return "", err
	}

	secBytes, ok := sec.Data[secretKey]
	if !ok {
		return "", fmt.Errorf("key %q not found in secret \"%s/%s\"", secretKey, secretName, namespace)
	}

	token := strings.TrimSpace(string(secBytes))
	if token == "" {
		return "", fmt.Errorf("secret \"%s/%s\" key %q: token is empty", secretName, namespace, secretKey)
	}

	return token, nil
}