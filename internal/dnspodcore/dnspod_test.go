package dnspodcore

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"strings"
	"testing"
)

func TestAuthorizationMatchesReferenceVector(t *testing.T) {
	payload := `{"Domain":"example.com","Limit":100,"Offset":0}`
	canonical := CanonicalRequest("dnspod.tencentcloudapi.com", payload)
	wantCanonical := strings.Join([]string{
		"POST",
		"/",
		"",
		"content-type:application/json; charset=utf-8",
		"host:dnspod.tencentcloudapi.com",
		"",
		"content-type;host",
		// sha256 of the payload above
		"fa95be13c2243b81be681919ce410e10c1929b5662dd6bbbae31b106451c9036",
	}, "\n")
	if canonical != wantCanonical {
		t.Fatalf("CanonicalRequest =\n%q\nwant\n%q", canonical, wantCanonical)
	}
	wantAuth := "TC3-HMAC-SHA256 Credential=test-id/2024-05-01/dnspod/tc3_request, SignedHeaders=content-type;host, Signature=eb45ffecd454e1916dd186b39ac5a7d22175206d3efb25bcfa14649aaf4f815e"
	if got := Authorization("test-id", "test-key", "dnspod.tencentcloudapi.com", payload, 1714521600); got != wantAuth {
		t.Fatalf("Authorization = %q", got)
	}
}

func TestPushReconcilesRecordsAtResolvedDomain(t *testing.T) {
	records := []apiRecord{
		{RecordID: 11, Name: "api", Type: "A", Value: "1.1.1.1", TTL: 600, Line: DefaultLine},
		{RecordID: 12, Name: "api", Type: "A", Value: "1.0.0.1", TTL: 600, Line: DefaultLine},
		{RecordID: 13, Name: "www", Type: "A", Value: "9.9.9.9", TTL: 600, Line: DefaultLine},
	}
	var created, updated, deleted int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.Header.Get("Authorization"), algorithm+" Credential=test-id/") {
			t.Errorf("unexpected authorization: %q", r.Header.Get("Authorization"))
		}
		action := r.Header.Get("X-TC-Action")
		body := decodeBody(t, r)
		switch action {
		case "DescribeDomainList":
			writeResponse(t, w, map[string]any{
				"DomainCountInfo": map[string]any{"AllTotal": 1},
				"DomainList":      []map[string]any{{"Domain": "example.com"}},
			})
		case "DescribeRecordList":
			if body["Domain"] != "example.com" {
				t.Errorf("Domain = %v", body["Domain"])
			}
			writeResponse(t, w, map[string]any{
				"RecordCountInfo": map[string]any{"TotalCount": len(records)},
				"RecordList":      records,
			})
		case "CreateRecord":
			created++
			if body["SubDomain"] != "api" || body["Domain"] != "example.com" || body["RecordLine"] != DefaultLine {
				t.Errorf("CreateRecord body: %#v", body)
			}
			id := int64(100 + created)
			records = append(records, apiRecord{RecordID: id, Name: "api", Type: str(body["RecordType"]), Value: str(body["Value"]), TTL: int(body["TTL"].(float64)), Line: DefaultLine})
			writeResponse(t, w, map[string]any{"RecordId": id})
		case "ModifyRecord":
			updated++
			writeResponse(t, w, map[string]any{"RecordId": body["RecordId"]})
		case "DeleteRecord":
			deleted++
			id := int64(body["RecordId"].(float64))
			filtered := make([]apiRecord, 0, len(records))
			for _, record := range records {
				if record.RecordID != id {
					filtered = append(filtered, record)
				}
			}
			records = filtered
			writeResponse(t, w, map[string]any{})
		default:
			t.Fatalf("unexpected action %q", action)
		}
	}))
	defer server.Close()

	client := NewClientWithOptions(ClientOptions{SecretID: "test-id", SecretKey: "test-key", BaseURL: server.URL})
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
		if record.Name == "api" {
			contents = append(contents, record.Value)
		}
		if record.Name == "www" && record.Value != "9.9.9.9" {
			t.Fatalf("unrelated record changed: %#v", record)
		}
	}
	slices.Sort(contents)
	if !slices.Equal(contents, []string{"2.2.2.2", "3.3.3.3"}) {
		t.Fatalf("api contents = %#v", contents)
	}
}

func TestPushUpdatesTTLInPlace(t *testing.T) {
	records := []apiRecord{{RecordID: 11, Name: "api", Type: "A", Value: "2.2.2.2", TTL: 3600, Line: DefaultLine}}
	var modifyBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := decodeBody(t, r)
		switch r.Header.Get("X-TC-Action") {
		case "DescribeDomainList":
			writeResponse(t, w, map[string]any{"DomainCountInfo": map[string]any{"AllTotal": 1}, "DomainList": []map[string]any{{"Domain": "example.com"}}})
		case "DescribeRecordList":
			writeResponse(t, w, map[string]any{"RecordCountInfo": map[string]any{"TotalCount": len(records)}, "RecordList": records})
		case "ModifyRecord":
			modifyBody = body
			writeResponse(t, w, map[string]any{"RecordId": body["RecordId"]})
		default:
			t.Fatalf("unexpected action %q", r.Header.Get("X-TC-Action"))
		}
	}))
	defer server.Close()

	client := NewClientWithOptions(ClientOptions{SecretID: "id", SecretKey: "key", BaseURL: server.URL})
	result, err := Push(context.Background(), client, Config{RecordName: "api.example.com", TTL: 600}, "2.2.2.2")
	if err != nil {
		t.Fatalf("Push returned error: %v", err)
	}
	if result.Summary.Updated != 1 || result.Summary.Created != 0 || result.Summary.Deleted != 0 {
		t.Fatalf("summary = %#v", result.Summary)
	}
	if strconv.FormatFloat(modifyBody["RecordId"].(float64), 'f', -1, 64) != "11" || strconv.FormatFloat(modifyBody["TTL"].(float64), 'f', -1, 64) != "600" {
		t.Fatalf("modify body = %#v", modifyBody)
	}
}

func TestListRecordsReportsAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, map[string]any{"Response": map[string]any{
			"Error":     map[string]any{"Code": "AuthFailure.SignatureFailure", "Message": "The provided credentials could not be validated."},
			"RequestId": "r9",
		}})
	}))
	defer server.Close()
	client := NewClientWithOptions(ClientOptions{SecretID: "id", SecretKey: "key", BaseURL: server.URL})
	_, err := client.ListRecords(context.Background(), Zone{Name: "example.com"})
	if err == nil {
		t.Fatal("ListRecords returned nil error")
	}
	if code := ErrorCode(err); code != "AuthFailure.SignatureFailure" {
		t.Fatalf("error code = %q", code)
	}
	if !strings.Contains(err.Error(), "The provided credentials could not be validated.") {
		t.Fatalf("error = %v", err)
	}
}

func writeResponse(t *testing.T, w http.ResponseWriter, payload map[string]any) {
	t.Helper()
	writeJSON(t, w, map[string]any{"Response": payload})
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
