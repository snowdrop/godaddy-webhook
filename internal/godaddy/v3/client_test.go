package v3

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
		AuthPAT: "test-pat-token",
		BaseURL: serverURL,
	}
}

func TestHasTXTRecord_Found(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v3/domains/zones/example.com/dns-records" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("type") != "TXT" {
			t.Errorf("expected type=TXT query param, got: %s", r.URL.Query().Get("type"))
		}
		if r.URL.Query().Get("name") != "_acme-challenge" {
			t.Errorf("expected name=_acme-challenge query param, got: %s", r.URL.Query().Get("name"))
		}
		if r.Method != http.MethodGet {
			t.Errorf("unexpected method: %s", r.Method)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(dnsRecordsListResponse{
			Items: []dnsRecordResponse{
				{RecordID: "rec-1", Type: "TXT", Name: "_acme-challenge", Data: "challenge-token"},
			},
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
		json.NewEncoder(w).Encode(dnsRecordsListResponse{Items: []dnsRecordResponse{}})
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
		json.NewEncoder(w).Encode(dnsRecordsListResponse{
			Items: []dnsRecordResponse{
				{RecordID: "rec-1", Type: "TXT", Name: "_acme-challenge", Data: "other-token"},
			},
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
		if r.URL.Path != "/v3/domains/zones/example.com/dns-records" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("unexpected method: %s", r.Method)
		}

		body, _ := io.ReadAll(r.Body)
		var record godaddy.DNSRecord
		if err := json.Unmarshal(body, &record); err != nil {
			t.Fatalf("failed to parse request body: %v", err)
		}
		if record.Data != "challenge-token" {
			t.Errorf("unexpected record data: %s", record.Data)
		}

		w.WriteHeader(http.StatusCreated)
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
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		switch requestCount {
		case 1:
			if r.URL.Path != "/v3/domains/zones/example.com/dns-records" {
				t.Errorf("unexpected list path: %s", r.URL.Path)
			}
			if r.Method != http.MethodGet {
				t.Errorf("expected GET for listing, got: %s", r.Method)
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(dnsRecordsListResponse{
				Items: []dnsRecordResponse{
					{RecordID: "rec-123", Type: "TXT", Name: "_acme-challenge", Data: "challenge-token"},
				},
			})
		case 2:
			if r.URL.Path != "/v3/domains/zones/example.com/dns-records/rec-123" {
				t.Errorf("unexpected delete path: %s", r.URL.Path)
			}
			if r.Method != http.MethodDelete {
				t.Errorf("expected DELETE, got: %s", r.Method)
			}
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Errorf("unexpected request #%d", requestCount)
		}
	}))
	defer server.Close()

	c := NewClient(testConfig(server.URL))
	err := c.DeleteTxtRecord("example.com", "_acme-challenge")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if requestCount != 2 {
		t.Errorf("expected 2 requests (list + delete), got %d", requestCount)
	}
}

func TestDeleteTxtRecord_NoRecords(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(dnsRecordsListResponse{Items: []dnsRecordResponse{}})
	}))
	defer server.Close()

	c := NewClient(testConfig(server.URL))
	err := c.DeleteTxtRecord("example.com", "_acme-challenge")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeleteTxtRecord_ListError(t *testing.T) {
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
