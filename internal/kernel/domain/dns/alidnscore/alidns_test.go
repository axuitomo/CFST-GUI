package alidnscore

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"testing"
)

func TestSignatureMatchesReferenceVector(t *testing.T) {
	client := NewClient("test-ak", "test-secret")
	params := map[string]string{
		"AccessKeyId":      "test-ak",
		"Action":           "DescribeDomainRecords",
		"DomainName":       "example.com",
		"Format":           "JSON",
		"PageNumber":       "1",
		"PageSize":         "500",
		"SignatureMethod":  "HMAC-SHA1",
		"SignatureNonce":   "1234567890",
		"SignatureVersion": "1.0",
		"Timestamp":        "2024-05-01T00:00:00Z",
		"Version":          "2015-01-09",
	}
	// 参考值由独立实现（net/url 转义 + HMAC-SHA1）算出，锁住排序、编码与签名串格式。
	if got := client.Sign(params); got != "hJW5CklS7apvmiJ2maSb9lBO9iI=" {
		t.Fatalf("Sign = %q", got)
	}
}

func TestPercentEncodeFollowsRFC3986(t *testing.T) {
	if got := percentEncode("a b*c~d/e+f"); got != "a%20b%2Ac~d%2Fe%2Bf" {
		t.Fatalf("percentEncode = %q", got)
	}
}

func TestPushReconcilesRecordsAtResolvedZone(t *testing.T) {
	records := []apiRecord{
		{RecordID: "11", RR: "api", Type: "A", Value: "1.1.1.1", TTL: 600, DomainName: "example.com"},
		{RecordID: "12", RR: "api", Type: "A", Value: "1.0.0.1", TTL: 600, DomainName: "example.com"},
		{RecordID: "13", RR: "www", Type: "A", Value: "9.9.9.9", TTL: 600, DomainName: "example.com"},
	}
	var created, updated, deleted int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		if query.Get("Signature") == "" || query.Get("SignatureMethod") != "HMAC-SHA1" || query.Get("Version") != apiVersion {
			t.Errorf("unexpected signature params: %v", query)
		}
		switch query.Get("Action") {
		case "DescribeDomains":
			writeJSON(t, w, map[string]any{
				"RequestId":  "r1",
				"TotalCount": 1,
				"Domains":    map[string]any{"Domain": []map[string]any{{"DomainName": "example.com"}}},
			})
		case "DescribeDomainRecords":
			if query.Get("DomainName") != "example.com" {
				t.Errorf("DomainName = %q", query.Get("DomainName"))
			}
			writeJSON(t, w, map[string]any{
				"RequestId":     "r2",
				"TotalCount":    len(records),
				"DomainRecords": map[string]any{"Record": records},
			})
		case "AddDomainRecord":
			created++
			if query.Get("RR") != "api" || query.Get("TTL") != "600" {
				t.Errorf("AddDomainRecord params: %v", query)
			}
			id := "100"
			records = append(records, apiRecord{RecordID: id, RR: "api", Type: query.Get("Type"), Value: query.Get("Value"), TTL: 600, DomainName: "example.com"})
			writeJSON(t, w, map[string]any{"RequestId": "r3", "RecordId": id})
		case "UpdateDomainRecord":
			updated++
			writeJSON(t, w, map[string]any{"RequestId": "r4", "RecordId": query.Get("RecordId")})
		case "DeleteDomainRecord":
			deleted++
			filtered := make([]apiRecord, 0, len(records))
			for _, record := range records {
				if record.RecordID != query.Get("RecordId") {
					filtered = append(filtered, record)
				}
			}
			records = filtered
			writeJSON(t, w, map[string]any{"RequestId": "r5"})
		default:
			t.Fatalf("unexpected action %q", query.Get("Action"))
		}
	}))
	defer server.Close()

	client := NewClientWithOptions(ClientOptions{AccessKeyID: "test-ak", AccessKeySecret: "s1", BaseURL: server.URL})
	result, err := Push(context.Background(), client, Config{RecordName: "api.example.com", TTL: 600}, "2.2.2.2\n3.3.3.3\nbad")
	if err != nil {
		t.Fatalf("Push returned error: %v", err)
	}
	if result.Summary.Created != 2 || result.Summary.Deleted != 2 || result.Summary.Ignored != 1 || result.Summary.Updated != 0 {
		t.Fatalf("summary = %#v", result.Summary)
	}
	if created != 2 || deleted != 2 || updated != 0 {
		t.Fatalf("operations = created %d updated %d deleted %d", created, updated, deleted)
	}
	contents := make([]string, 0)
	for _, record := range records {
		if record.RR == "api" {
			contents = append(contents, record.Value)
		}
		if record.RR == "www" && record.Value != "9.9.9.9" {
			t.Fatalf("unrelated record changed: %#v", record)
		}
	}
	slices.Sort(contents)
	if !slices.Equal(contents, []string{"2.2.2.2", "3.3.3.3"}) {
		t.Fatalf("api contents = %#v", contents)
	}
}

func TestPushUpdatesTTLInPlace(t *testing.T) {
	records := []apiRecord{{RecordID: "11", RR: "api", Type: "A", Value: "2.2.2.2", TTL: 3600, DomainName: "example.com"}}
	var updateQuery url.Values
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		switch query.Get("Action") {
		case "DescribeDomains":
			writeJSON(t, w, map[string]any{"TotalCount": 1, "Domains": map[string]any{"Domain": []map[string]any{{"DomainName": "example.com"}}}})
		case "DescribeDomainRecords":
			writeJSON(t, w, map[string]any{"TotalCount": len(records), "DomainRecords": map[string]any{"Record": records}})
		case "UpdateDomainRecord":
			updateQuery = query
			writeJSON(t, w, map[string]any{"RecordId": query.Get("RecordId")})
		default:
			t.Fatalf("unexpected action %q", query.Get("Action"))
		}
	}))
	defer server.Close()

	client := NewClientWithOptions(ClientOptions{AccessKeyID: "ak", AccessKeySecret: "secret", BaseURL: server.URL})
	result, err := Push(context.Background(), client, Config{AllowCNAME: true, RecordName: "api.example.com", TTL: 600}, "2.2.2.2")
	if err != nil {
		t.Fatalf("Push returned error: %v", err)
	}
	if result.Summary.Updated != 1 || result.Summary.Created != 0 || result.Summary.Deleted != 0 {
		t.Fatalf("summary = %#v", result.Summary)
	}
	if updateQuery.Get("RecordId") != "11" || updateQuery.Get("RR") != "api" || updateQuery.Get("TTL") != "600" {
		t.Fatalf("update params = %v", updateQuery)
	}
}

func TestPushReplacesBlockingCNAMERecord(t *testing.T) {
	records := []apiRecord{{RecordID: "5", RR: "api", Type: "CNAME", Value: "origin.example.net", TTL: 600, DomainName: "example.com"}}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		switch query.Get("Action") {
		case "DescribeDomains":
			writeJSON(t, w, map[string]any{"TotalCount": 1, "Domains": map[string]any{"Domain": []map[string]any{{"DomainName": "example.com"}}}})
		case "DescribeDomainRecords":
			writeJSON(t, w, map[string]any{"TotalCount": len(records), "DomainRecords": map[string]any{"Record": records}})
		case "DeleteDomainRecord":
			records = nil
			writeJSON(t, w, map[string]any{})
		case "AddDomainRecord":
			records = append(records, apiRecord{RecordID: "200", RR: "api", Type: query.Get("Type"), Value: query.Get("Value"), TTL: 600})
			writeJSON(t, w, map[string]any{"RecordId": "200"})
		default:
			t.Fatalf("unexpected action %q", query.Get("Action"))
		}
	}))
	defer server.Close()

	client := NewClientWithOptions(ClientOptions{AccessKeyID: "ak", AccessKeySecret: "secret", BaseURL: server.URL})
	result, err := Push(context.Background(), client, Config{AllowCNAME: true, RecordName: "api.example.com", TTL: 600}, "2.2.2.2")
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

func TestListZonesReportsAuthError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		writeJSON(t, w, map[string]any{"Code": "InvalidAccessKeyId.NotFound", "Message": "Specified access key is not found.", "RequestId": "r9"})
	}))
	defer server.Close()
	client := NewClientWithOptions(ClientOptions{AccessKeyID: "ak", AccessKeySecret: "secret", BaseURL: server.URL})
	_, err := client.ListZones(context.Background())
	if err == nil {
		t.Fatal("ListZones returned nil error")
	}
	if code := ErrorCode(err); code != "InvalidAccessKeyId.NotFound" {
		t.Fatalf("error code = %q", code)
	}
	if !strings.Contains(err.Error(), "Specified access key is not found.") {
		t.Fatalf("error = %v", err)
	}
}

func writeJSON(t *testing.T, w http.ResponseWriter, payload map[string]any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		t.Fatalf("encode response: %v", err)
	}
}
