package dnshecore

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestPushReconcilesRecordsAtResolvedSubdomain(t *testing.T) {
	records := []apiRecord{
		{ID: 11, Name: "edge.example.com", Type: "A", Content: "1.1.1.1", TTL: 600},
		{ID: 12, Name: "edge.example.com", Type: "A", Content: "1.0.0.1", TTL: 600},
		{ID: 13, Name: "other.example.com", Type: "A", Content: "9.9.9.9", TTL: 600},
	}
	var created, updated, deleted int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-API-Key") != "key-1" || r.Header.Get("X-API-Secret") != "s1" {
			t.Errorf("missing auth headers: %#v", r.Header)
		}
		endpoint := r.URL.Query().Get("endpoint")
		action := r.URL.Query().Get("action")
		switch endpoint + "." + action {
		case "subdomains.list":
			writeJSON(t, w, map[string]any{"success": true, "subdomains": []map[string]any{
				{"id": 7, "subdomain": "example", "rootdomain": "com", "full_domain": "example.com"},
				{"id": 8, "subdomain": "edge", "rootdomain": "example.com", "full_domain": "edge.example.com"},
			}})
		case "dns_records.list":
			if r.URL.Query().Get("subdomain_id") != "8" {
				t.Errorf("subdomain_id = %s, want 8", r.URL.Query().Get("subdomain_id"))
			}
			writeJSON(t, w, map[string]any{"success": true, "records": records})
		case "dns_records.create":
			created++
			body := decodeBody(t, r)
			if body["subdomain_id"] != float64(8) {
				t.Errorf("create subdomain_id = %v", body["subdomain_id"])
			}
			id := 100 + created
			record := apiRecord{ID: id, Name: "edge.example.com", Type: str(body["type"]), Content: str(body["content"]), TTL: int(body["ttl"].(float64))}
			records = append(records, record)
			writeJSON(t, w, map[string]any{"success": true, "id": id, "record_id": "provider-" + strconv.Itoa(id)})
		case "dns_records.update":
			updated++
			writeJSON(t, w, map[string]any{"success": true})
		case "dns_records.delete":
			deleted++
			body := decodeBody(t, r)
			id := int(body["id"].(float64))
			filtered := make([]apiRecord, 0, len(records))
			for _, record := range records {
				if record.ID != id {
					filtered = append(filtered, record)
				}
			}
			records = filtered
			writeJSON(t, w, map[string]any{"success": true})
		default:
			t.Fatalf("unexpected request %s %s", endpoint, action)
		}
	}))
	defer server.Close()

	client := NewClientWithOptions(ClientOptions{APIKey: "key-1", APISecret: "s1", BaseURL: server.URL})
	cfg := Config{RecordName: "edge.example.com", RecordType: RecordTypeA, TTL: 600}
	result, err := Push(context.Background(), client, cfg, "2.2.2.2\n3.3.3.3\nbad")
	if err != nil {
		t.Fatalf("Push returned error: %v", err)
	}
	if result.Summary.Created != 2 || result.Summary.Deleted != 2 || result.Summary.Ignored != 1 {
		t.Fatalf("summary = %#v", result.Summary)
	}
	if created != 2 || updated != 0 || deleted != 2 {
		t.Fatalf("operations = created %d updated %d deleted %d", created, updated, deleted)
	}
	contents := make([]string, 0, len(records))
	for _, record := range records {
		if record.Name == "edge.example.com" {
			contents = append(contents, record.Content)
		}
	}
	slices.Sort(contents)
	if !slices.Equal(contents, []string{"2.2.2.2", "3.3.3.3"}) {
		t.Fatalf("edge contents = %#v", contents)
	}
}

func TestPushReplacesBlockingCNAMERecord(t *testing.T) {
	records := []apiRecord{{ID: 5, Name: "edge.example.com", Type: "CNAME", Content: "origin.example.net", TTL: 600}}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("endpoint") + "." + r.URL.Query().Get("action") {
		case "subdomains.list":
			writeJSON(t, w, map[string]any{"success": true, "subdomains": []map[string]any{
				{"id": 8, "subdomain": "edge", "rootdomain": "example.com", "full_domain": "edge.example.com"},
			}})
		case "dns_records.list":
			writeJSON(t, w, map[string]any{"success": true, "records": records})
		case "dns_records.create":
			body := decodeBody(t, r)
			records = append(records, apiRecord{ID: 200, Name: "edge.example.com", Type: str(body["type"]), Content: str(body["content"]), TTL: 600})
			writeJSON(t, w, map[string]any{"success": true})
		case "dns_records.delete":
			body := decodeBody(t, r)
			id := int(body["id"].(float64))
			filtered := make([]apiRecord, 0)
			for _, record := range records {
				if record.ID != id {
					filtered = append(filtered, record)
				}
			}
			records = filtered
			writeJSON(t, w, map[string]any{"success": true})
		default:
			t.Fatalf("unexpected request %s", r.URL.RawQuery)
		}
	}))
	defer server.Close()

	client := NewClientWithOptions(ClientOptions{APIKey: "key-1", APISecret: "s1", BaseURL: server.URL})
	result, err := Push(context.Background(), client, Config{AllowCNAME: true, RecordName: "edge.example.com", TTL: 600}, "2.2.2.2")
	if err != nil {
		t.Fatalf("Push returned error: %v", err)
	}
	if result.Summary.Created != 1 || result.Summary.Deleted != 1 {
		t.Fatalf("summary = %#v", result.Summary)
	}
	for _, record := range records {
		if record.Type == "CNAME" {
			t.Fatalf("CNAME record still present: %#v", records)
		}
	}
}

func TestResolveSubdomainPrefersLongestSuffix(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, map[string]any{"success": true, "subdomains": []map[string]any{
			{"id": 1, "full_domain": "example.com"},
			{"id": 2, "full_domain": "edge.example.com"},
		}})
	}))
	defer server.Close()
	client := NewClientWithOptions(ClientOptions{APIKey: "k", APISecret: "s", BaseURL: server.URL})
	subdomain, err := client.ResolveSubdomain(context.Background(), "www.edge.example.com")
	if err != nil {
		t.Fatalf("ResolveSubdomain returned error: %v", err)
	}
	if subdomain.ID != 2 {
		t.Fatalf("subdomain ID = %d, want 2", subdomain.ID)
	}
	if got := relativeRecordName(subdomain.FullDomain, "www.edge.example.com"); got != "www" {
		t.Fatalf("relative name = %q, want www", got)
	}
	if got := relativeRecordName(subdomain.FullDomain, "edge.example.com"); got != "@" {
		t.Fatalf("relative name = %q, want @", got)
	}
}

func TestListSubdomainsReportsAuthError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		writeJSON(t, w, map[string]any{"success": false, "error_code": "auth_invalid_credentials", "message": "Invalid API key"})
	}))
	defer server.Close()
	client := NewClientWithOptions(ClientOptions{APIKey: "k", APISecret: "s", BaseURL: server.URL})
	_, err := client.ListSubdomains(context.Background())
	if err == nil {
		t.Fatal("ListSubdomains returned nil error")
	}
	if ErrorCode(err) != "" && ErrorCode(err) != "auth_invalid_credentials" {
		t.Fatalf("error code = %q", ErrorCode(err))
	}
	if !strings.Contains(err.Error(), "Invalid API key") {
		t.Fatalf("error = %v", err)
	}
}

func TestResolveSubdomainByNameAndRejectMismatchedRecord(t *testing.T) {
	created := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("endpoint") + "." + r.URL.Query().Get("action") {
		case "subdomains.list":
			writeJSON(t, w, map[string]any{"success": true, "subdomains": []map[string]any{
				{"id": 7, "subdomain": "api", "rootdomain": "l.cd", "full_domain": "api.l.cd"},
				{"id": 9, "subdomain": "edge", "rootdomain": "l.cd", "full_domain": "edge.l.cd"},
			}})
		case "dns_records.list":
			if r.URL.Query().Get("subdomain_id") != "9" {
				t.Errorf("subdomain_id = %s, want 9", r.URL.Query().Get("subdomain_id"))
			}
			writeJSON(t, w, map[string]any{"success": true, "records": []map[string]any{}})
		case "dns_records.create":
			created++
			writeJSON(t, w, map[string]any{"success": true, "id": 40})
		default:
			t.Fatalf("unexpected request %s", r.URL.RawQuery)
		}
	}))
	defer server.Close()

	client := NewClientWithOptions(ClientOptions{APIKey: "key-1", APISecret: "s1", BaseURL: server.URL})
	subdomain, err := client.ResolveSubdomainByName(context.Background(), "edge")
	if err != nil || subdomain.ID != 9 || subdomain.FullDomain != "edge.l.cd" {
		t.Fatalf("ResolveSubdomainByName = %#v, %v", subdomain, err)
	}

	_, err = Push(context.Background(), client, Config{Subdomain: "edge", RecordName: "api.l.cd", TTL: 600}, "2.2.2.2")
	if err == nil || !strings.Contains(err.Error(), "不属于子域名") {
		t.Fatalf("mismatched record name error = %v", err)
	}
	if created != 0 {
		t.Fatalf("mismatched record name created %d records, want 0", created)
	}
}

func TestPushUsesSelectedSubdomainApex(t *testing.T) {
	var createdName string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("endpoint") + "." + r.URL.Query().Get("action") {
		case "subdomains.list":
			writeJSON(t, w, map[string]any{"success": true, "subdomains": []map[string]any{
				{"id": 9, "subdomain": "edge", "rootdomain": "l.cd", "full_domain": "edge.l.cd"},
			}})
		case "dns_records.list":
			writeJSON(t, w, map[string]any{"success": true, "records": []map[string]any{}})
		case "dns_records.create":
			body := decodeBody(t, r)
			createdName = str(body["name"])
			writeJSON(t, w, map[string]any{"success": true, "id": 40})
		default:
			t.Fatalf("unexpected request %s", r.URL.RawQuery)
		}
	}))
	defer server.Close()

	client := NewClientWithOptions(ClientOptions{APIKey: "key-1", APISecret: "s1", BaseURL: server.URL})
	result, err := Push(context.Background(), client, Config{Subdomain: "edge", TTL: 600}, "2.2.2.2")
	if err != nil {
		t.Fatalf("Push returned error: %v", err)
	}
	if result.Summary.Created != 1 || createdName != "@" {
		t.Fatalf("summary = %#v, created name = %q", result.Summary, createdName)
	}
}

func TestResolveSubdomainTargetComposesBareLabel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, map[string]any{"success": true, "subdomains": []map[string]any{
			{"id": 5, "subdomain": "axuitomo", "rootdomain": "l.cd", "full_domain": "axuitomo.l.cd"},
		}})
	}))
	defer server.Close()

	client := NewClientWithOptions(ClientOptions{APIKey: "key", APISecret: "secret", BaseURL: server.URL})
	subdomain, recordName, err := client.ResolveSubdomainTarget(context.Background(), "test", "axuitomo.l.cd")
	if err != nil {
		t.Fatalf("ResolveSubdomainTarget returned error: %v", err)
	}
	if subdomain.ID != 5 || subdomain.FullDomain != "axuitomo.l.cd" {
		t.Fatalf("subdomain = %#v, want axuitomo.l.cd (id 5)", subdomain)
	}
	if recordName != "test.axuitomo.l.cd" {
		t.Fatalf("recordName = %q, want test.axuitomo.l.cd", recordName)
	}
}

func TestPushTreatsBareSubdomainLabelAsHostRecord(t *testing.T) {
	var pushedName string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("endpoint") + "." + r.URL.Query().Get("action") {
		case "subdomains.list":
			writeJSON(t, w, map[string]any{"success": true, "subdomains": []map[string]any{
				{"id": 5, "subdomain": "axuitomo", "rootdomain": "l.cd", "full_domain": "axuitomo.l.cd"},
			}})
		case "dns_records.list":
			if r.URL.Query().Get("subdomain_id") != "5" {
				t.Errorf("subdomain_id = %s, want 5", r.URL.Query().Get("subdomain_id"))
			}
			writeJSON(t, w, map[string]any{"success": true, "records": []map[string]any{}})
		case "dns_records.create":
			body := decodeBody(t, r)
			pushedName = str(body["name"])
			writeJSON(t, w, map[string]any{"success": true, "id": 41})
		default:
			t.Fatalf("unexpected request %s", r.URL.RawQuery)
		}
	}))
	defer server.Close()

	client := NewClientWithOptions(ClientOptions{APIKey: "key-1", APISecret: "s1", BaseURL: server.URL})
	result, err := Push(context.Background(), client, Config{Subdomain: "test", RecordName: "axuitomo.l.cd", TTL: 600}, "2.2.2.2")
	if err != nil {
		t.Fatalf("Push returned error: %v", err)
	}
	if result.Summary.Created != 1 || pushedName != "test" {
		t.Fatalf("summary = %#v, created name = %q", result.Summary, pushedName)
	}
}

func writeJSON(t *testing.T, w http.ResponseWriter, payload map[string]any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		t.Fatalf("encode response: %v", err)
	}
}

func decodeBody(t *testing.T, r *http.Request) map[string]any {
	t.Helper()
	defer r.Body.Close()
	body := map[string]any{}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	return body
}

func str(value any) string {
	text, _ := value.(string)
	return text
}

func TestClientRetries429WithBackoff(t *testing.T) {
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if attempts.Add(1) <= 2 {
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"success":false,"message":"rate limited"}`))
			return
		}
		writeJSON(t, w, map[string]any{"success": true, "subdomain": map[string]any{
			"id": 8, "subdomain": "edge", "rootdomain": "example.com", "full_domain": "edge.example.com",
		}})
	}))
	defer server.Close()

	client := NewClientWithOptions(ClientOptions{APIKey: "key", APISecret: "secret", BaseURL: server.URL})
	client.retryBaseDelay = time.Millisecond
	subdomain, err := client.GetSubdomain(context.Background(), 8)
	if err != nil {
		t.Fatalf("GetSubdomain returned error after retries: %v", err)
	}
	if subdomain.FullDomain != "edge.example.com" {
		t.Fatalf("subdomain = %#v", subdomain)
	}
	if got := attempts.Load(); got != 3 {
		t.Fatalf("attempts = %d, want 3 (two 429 responses then success)", got)
	}
}

func TestClientStopsAfterRetryBudgetWhenRateLimited(t *testing.T) {
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"success":false,"message":"rate limited"}`))
	}))
	defer server.Close()

	client := NewClientWithOptions(ClientOptions{APIKey: "key", APISecret: "secret", BaseURL: server.URL})
	client.retryBaseDelay = time.Millisecond
	_, err := client.GetSubdomain(context.Background(), 8)
	if err == nil || !strings.Contains(err.Error(), "429") {
		t.Fatalf("error = %v, want 429 rate-limit error", err)
	}
	if got := attempts.Load(); got != 4 {
		t.Fatalf("attempts = %d, want 4 (initial + 3 retries)", got)
	}
}

func TestSetRequestIntervalSpacesCalls(t *testing.T) {
	client := NewClientWithOptions(ClientOptions{APIKey: "key", APISecret: "secret"})
	client.SetRequestInterval(20 * time.Millisecond)
	if err := client.waitRequestInterval(context.Background()); err != nil {
		t.Fatalf("first waitRequestInterval: %v", err)
	}
	started := time.Now()
	if err := client.waitRequestInterval(context.Background()); err != nil {
		t.Fatalf("second waitRequestInterval: %v", err)
	}
	if elapsed := time.Since(started); elapsed < 15*time.Millisecond {
		t.Fatalf("second call returned after %v, want ~20ms", elapsed)
	}
}
