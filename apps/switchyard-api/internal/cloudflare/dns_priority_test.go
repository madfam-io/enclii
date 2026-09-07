package cloudflare

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// The MX apply that answered 502 (enclii#530).
//
// CreateDNSRecordInZone marshalled a fixed struct with no `priority` field at
// all, so an MX create reached Cloudflare with type/name/content/proxied/ttl
// and nothing else. Cloudflare requires a priority for MX and rejects the
// request; that rejection then surfaced as a gateway error rather than as the
// record problem it is. An MX create must always carry a priority.
func TestCreateMXAlwaysSendsPriority(t *testing.T) {
	var payload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &payload)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"errors":[],"messages":[],"result":{"id":"rec_1","type":"MX"}}`))
	}))
	defer server.Close()

	client := newTestClient(t, server)
	if _, err := client.CreateDNSRecordInZone(context.Background(),
		"zone", "example.com", "MX", "mail.example.com", false); err != nil {
		t.Fatalf("create failed: %v", err)
	}

	got, ok := payload["priority"]
	if !ok {
		t.Fatal("an MX create with no explicit priority must still send one — omitting it is the Cloudflare 400 behind the #530 502")
	}
	if int(got.(float64)) != RecordTypeDefaultPriority {
		t.Fatalf("expected the documented default priority %d, got %v", RecordTypeDefaultPriority, got)
	}
}

// A caller-supplied priority is sent verbatim, including 0 — which is a legal
// MX preference and used to be swallowed by a `priority > 0` guard.
func TestCreateMXSendsExplicitPriorityIncludingZero(t *testing.T) {
	for _, want := range []int{0, 10, 20, 65535} {
		var payload map[string]any
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(body, &payload)
			_, _ = w.Write([]byte(`{"success":true,"errors":[],"messages":[],"result":{"id":"rec_1"}}`))
		}))

		client := newTestClient(t, server)
		if _, err := client.CreateDNSRecordInZoneWithPriority(context.Background(),
			"zone", "example.com", "MX", "mail.example.com", false, want); err != nil {
			t.Fatalf("create failed: %v", err)
		}
		server.Close()

		got, ok := payload["priority"]
		if !ok {
			t.Fatalf("priority %d was dropped from the payload", want)
		}
		if int(got.(float64)) != want {
			t.Fatalf("priority %d was sent as %v", want, got)
		}
	}
}

// A type with no priority must not grow one: Cloudflare rejects a priority on
// a CNAME/TXT.
func TestCreateNonPriorityTypeSendsNoPriority(t *testing.T) {
	for _, recordType := range []string{"CNAME", "TXT", "A"} {
		var payload map[string]any
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(body, &payload)
			_, _ = w.Write([]byte(`{"success":true,"errors":[],"messages":[],"result":{"id":"rec_1"}}`))
		}))

		client := newTestClient(t, server)
		if _, err := client.CreateDNSRecordInZoneWithPriority(context.Background(),
			"zone", "example.com", recordType, "value", false, 10); err != nil {
			t.Fatalf("create failed: %v", err)
		}
		server.Close()

		if _, ok := payload["priority"]; ok {
			t.Fatalf("%s must not carry a priority, got %v", recordType, payload["priority"])
		}
	}
}

// Cloudflare's PUT replaces the whole record, so an MX update that omits
// priority is a 400 rather than a partial edit. The record's own priority is
// preserved when the caller supplies none.
func TestUpdateMXPreservesPriority(t *testing.T) {
	var payload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &payload)
		_, _ = w.Write([]byte(`{"success":true,"errors":[],"messages":[],"result":{"id":"rec_1"}}`))
	}))
	defer server.Close()

	existing := 20
	record := DNSRecord{ID: "rec_1", Name: "example.com", Type: "MX", Priority: &existing, TTL: 1}

	client := newTestClient(t, server)
	if _, err := client.UpdateDNSRecordInZone(context.Background(), "zone", record, "mailsec.example.com", false); err != nil {
		t.Fatalf("update failed: %v", err)
	}

	got, ok := payload["priority"]
	if !ok {
		t.Fatal("an MX update must carry a priority — Cloudflare's PUT replaces the record")
	}
	if int(got.(float64)) != existing {
		t.Fatalf("expected the record's own priority %d, got %v", existing, got)
	}
}

// ListDNSRecordsByTypeInZone must return every record at the name, not just
// the first. Reading only the first is what let one TXT stand in for a set.
func TestListDNSRecordsByTypeReturnsAll(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"success":true,"errors":[],"messages":[],"result":[
			{"id":"rec_1","type":"TXT","name":"example.com","content":"protonmail-verification=abc"},
			{"id":"rec_2","type":"TXT","name":"example.com","content":"v=spf1 include:_spf.protonmail.ch ~all"}
		]}`))
	}))
	defer server.Close()

	client := newTestClient(t, server)
	records, err := client.ListDNSRecordsByTypeInZone(context.Background(), "zone", "example.com", "TXT")
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("both TXT records at the name must be returned, got %d", len(records))
	}

	// The single-record accessor still answers the first, for callers that
	// legitimately only handle one.
	single, err := client.GetDNSRecordByTypeInZone(context.Background(), "zone", "example.com", "TXT")
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}
	if single == nil || single.ID != "rec_1" {
		t.Fatalf("the single-record accessor must keep returning the first, got %+v", single)
	}
}

// A Cloudflare 400 must reach the caller as a typed APIError carrying
// Cloudflare's own message, so the handler can map it to a 4xx instead of
// answering an opaque gateway error.
func TestCreateSurfacesCloudflareRejectionAsAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"success":false,"errors":[{"code":9007,"message":"Content for a MX record must be a valid hostname"}],"messages":[],"result":null}`))
	}))
	defer server.Close()

	client := newTestClient(t, server)
	_, err := client.CreateDNSRecordInZone(context.Background(), "zone", "example.com", "MX", "10 mail.example.com", false)
	if err == nil {
		t.Fatal("a rejected record must be an error")
	}

	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("the Cloudflare error must survive wrapping as a typed APIError, got %T: %v", err, err)
	}
	if apiErr.Code != 9007 {
		t.Fatalf("expected Cloudflare code 9007, got %d", apiErr.Code)
	}
}

func TestRecordTypeRequiresPriority(t *testing.T) {
	for _, tt := range []string{"MX", "mx", " SRV ", "URI"} {
		if !RecordTypeRequiresPriority(tt) {
			t.Fatalf("%q requires a priority", tt)
		}
	}
	for _, tt := range []string{"CNAME", "TXT", "A", "AAAA", "NS", ""} {
		if RecordTypeRequiresPriority(tt) {
			t.Fatalf("%q must not carry a priority", tt)
		}
	}
}
