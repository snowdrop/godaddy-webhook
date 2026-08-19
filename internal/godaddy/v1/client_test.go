package v1

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/snowdrop/godaddy-webhook/internal/godaddy"
)

func testConfig(serverURL string) *godaddy.ClientConfig {
	return &godaddy.ClientConfig{
		AuthAPIKey:    "testkey",
		AuthAPISecret: "testsecret",
		BaseURL:       serverURL,
	}
}

func TestHasTXTRecord_Found(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/domains/example.com/records/TXT/_acme-challenge" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodGet {
			t.Errorf("unexpected method: %s", r.Method)
		}
		if r.Header.Get("Authorization") != "sso-key testkey:testsecret" {
			t.Errorf("unexpected auth header: %s", r.Header.Get("Authorization"))
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]godaddy.DNSRecord{
			{Type: "TXT", Name: "_acme-challenge", Data: "challenge-token"},
		})
	}))
	defer server.Close()

	c := NewClient(testConfig(server.URL))
	found, err := c.HasTXTRecord("example.com", "_acme-challenge", "challenge-token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !found {
		t.Error("expected record to be found")
	}
}

func TestHasTXTRecord_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	c := NewClient(testConfig(server.URL))
	found, err := c.HasTXTRecord("example.com", "_acme-challenge", "challenge-token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if found {
		t.Error("expected record not to be found")
	}
}

func TestHasTXTRecord_EmptyResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]godaddy.DNSRecord{})
	}))
	defer server.Close()

	c := NewClient(testConfig(server.URL))
	found, err := c.HasTXTRecord("example.com", "_acme-challenge", "challenge-token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if found {
		t.Error("expected record not to be found for empty response")
	}
}

func TestHasTXTRecord_WrongKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]godaddy.DNSRecord{
			{Type: "TXT", Name: "_acme-challenge", Data: "other-token"},
		})
	}))
	defer server.Close()

	c := NewClient(testConfig(server.URL))
	found, err := c.HasTXTRecord("example.com", "_acme-challenge", "challenge-token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if found {
		t.Error("expected record not to match different challenge key")
	}
}

func TestUpdateRecords(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/domains/example.com/records/TXT/_acme-challenge" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPut {
			t.Errorf("unexpected method: %s", r.Method)
		}

		body, _ := io.ReadAll(r.Body)
		var records []godaddy.DNSRecord
		if err := json.Unmarshal(body, &records); err != nil {
			t.Fatalf("failed to parse request body: %v", err)
		}
		if len(records) != 1 || records[0].Data != "challenge-token" {
			t.Errorf("unexpected records: %+v", records)
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	c := NewClient(testConfig(server.URL))
	err := c.UpdateRecords([]godaddy.DNSRecord{
		{Type: "TXT", Name: "_acme-challenge", Data: "challenge-token", TTL: 600},
	}, "example.com", "_acme-challenge")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUpdateRecords_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"message":"internal error"}`))
	}))
	defer server.Close()

	c := NewClient(testConfig(server.URL))
	err := c.UpdateRecords([]godaddy.DNSRecord{
		{Type: "TXT", Name: "_acme-challenge", Data: "challenge-token"},
	}, "example.com", "_acme-challenge")
	if err == nil {
		t.Fatal("expected error for server error response")
	}
}

func TestDeleteTxtRecord(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/domains/example.com/records/TXT/_acme-challenge" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodDelete {
			t.Errorf("unexpected method: %s", r.Method)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	c := NewClient(testConfig(server.URL))
	err := c.DeleteTxtRecord("example.com", "_acme-challenge")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeleteTxtRecord_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	c := NewClient(testConfig(server.URL))
	err := c.DeleteTxtRecord("example.com", "_acme-challenge")
	if err == nil {
		t.Fatal("expected error for server error response")
	}
}
