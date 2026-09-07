package api

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/madfam-org/enclii/apps/switchyard-api/internal/cloudflare"
)

func intPtr(v int) *int { return &v }

func txt(id, content string) cloudflare.DNSRecord {
	return cloudflare.DNSRecord{ID: id, Name: "creatumundo.mx", Type: "TXT", Content: content}
}

func mx(id, content string, priority int) cloudflare.DNSRecord {
	return cloudflare.DNSRecord{ID: id, Name: "creatumundo.mx", Type: "MX", Content: content, Priority: intPtr(priority)}
}

// The regression from enclii#530, stated as a test.
//
// On 2026-09-07 an SPF TXT was applied to an apex that already held a Proton
// ownership TXT. The plan said `create`; the apply issued an `update` and the
// ownership record ceased to exist. Different content at the same name+type is
// a DIFFERENT record for a multi-value type, and the only non-destructive plan
// for it is a create.
func TestPlanSecondTXTAtSameNameIsCreateNotUpdate(t *testing.T) {
	live := []cloudflare.DNSRecord{txt("rec_proton", "protonmail-verification=abc123")}
	intent := cloudflareDNSApplyIntent{
		Target:     "creatumundo.mx",
		RecordType: "TXT",
		Content:    "v=spf1 include:_spf.protonmail.ch ~all",
	}

	plan := planCloudflareDNSApply(intent, live)

	if plan.Mutation != "create" {
		t.Fatalf("second TXT at one name must be a create, got %q — this is the #530 regression", plan.Mutation)
	}
	if plan.Match != nil {
		t.Fatalf("a create must not name a record to overwrite, got %+v", plan.Match)
	}
	if len(plan.Siblings) != 1 || plan.Siblings[0].ID != "rec_proton" {
		t.Fatalf("the existing TXT must be reported as an untouched sibling, got %+v", plan.Siblings)
	}
	if len(plan.Warnings) == 0 {
		t.Fatal("an add that joins existing records must say so")
	}
}

// The same shape for MX: the standard provider pair (10 primary + 20 backup)
// must be expressible.
func TestPlanSecondMXAtSameNameIsCreate(t *testing.T) {
	live := []cloudflare.DNSRecord{mx("rec_primary", "mail.protonmail.ch", 10)}
	intent := cloudflareDNSApplyIntent{
		Target:     "creatumundo.mx",
		RecordType: "MX",
		Content:    "mailsec.protonmail.ch",
		Priority:   intPtr(20),
	}

	plan := planCloudflareDNSApply(intent, live)

	if plan.Mutation != "create" {
		t.Fatalf("the second MX of a provider pair must be a create, got %q", plan.Mutation)
	}
	if len(plan.Siblings) != 1 {
		t.Fatalf("the primary MX must survive as a sibling, got %+v", plan.Siblings)
	}
}

// Idempotency: re-applying a record that already exists exactly must not
// rewrite it, whichever position it holds in the live set.
func TestPlanExactMatchIsNoop(t *testing.T) {
	live := []cloudflare.DNSRecord{
		txt("rec_proton", "protonmail-verification=abc123"),
		txt("rec_spf", `"v=spf1 include:_spf.protonmail.ch ~all"`),
	}
	intent := cloudflareDNSApplyIntent{
		Target:     "creatumundo.mx",
		RecordType: "TXT",
		Content:    "v=spf1 include:_spf.protonmail.ch ~all",
	}

	plan := planCloudflareDNSApply(intent, live)

	if plan.Mutation != "noop" {
		t.Fatalf("an identical record must be a noop, got %q", plan.Mutation)
	}
	if plan.Match == nil || plan.Match.ID != "rec_spf" {
		t.Fatalf("the noop must name the matching record, got %+v", plan.Match)
	}
	// Cloudflare returns TXT content wrapped in quotes the operator never
	// typed. Comparing raw strings would rewrite an identical record forever.
	if len(plan.Warnings) != 0 {
		t.Fatalf("a noop needs no warnings, got %v", plan.Warnings)
	}
}

// CNAME semantics are unchanged: a name holds exactly one CNAME, so changing
// it is an update of the record that is there.
func TestPlanCNAMEStillUpdates(t *testing.T) {
	live := []cloudflare.DNSRecord{{
		ID: "rec_cname", Name: "app.example.com", Type: "CNAME",
		Content: "old.cfargotunnel.com", Proxied: true,
	}}
	intent := cloudflareDNSApplyIntent{
		Target:     "app.example.com",
		RecordType: "CNAME",
		Content:    "new.cfargotunnel.com",
		Proxied:    true,
	}

	plan := planCloudflareDNSApply(intent, live)

	if plan.Mutation != "update" {
		t.Fatalf("a CNAME repoint must stay an update, got %q", plan.Mutation)
	}
	if plan.Match == nil || plan.Match.ID != "rec_cname" {
		t.Fatalf("the update must target the existing CNAME, got %+v", plan.Match)
	}
}

// A is deliberately kept on single-record update semantics: turning a host
// repoint into a second A record serving the old address is a worse failure
// than the one #530 fixes.
func TestPlanARecordStillUpdates(t *testing.T) {
	live := []cloudflare.DNSRecord{{ID: "rec_a", Name: "a.example.com", Type: "A", Content: "1.2.3.4"}}
	intent := cloudflareDNSApplyIntent{Target: "a.example.com", RecordType: "A", Content: "5.6.7.8"}

	if plan := planCloudflareDNSApply(intent, live); plan.Mutation != "update" {
		t.Fatalf("A repoint must stay an update, got %q", plan.Mutation)
	}
}

// Empty zone: nothing to collide with.
func TestPlanNoLiveRecordsIsCreate(t *testing.T) {
	intent := cloudflareDNSApplyIntent{Target: "creatumundo.mx", RecordType: "TXT", Content: "anything"}
	plan := planCloudflareDNSApply(intent, nil)
	if plan.Mutation != "create" {
		t.Fatalf("no live record must be a create, got %q", plan.Mutation)
	}
	if len(plan.Warnings) != 0 {
		t.Fatalf("a create into an empty name needs no warnings, got %v", plan.Warnings)
	}
}

// --replace is the escape hatch, and it must be loud: it is the flag that
// re-enables the destructive behaviour.
func TestPlanReplaceOverwritesAndWarns(t *testing.T) {
	live := []cloudflare.DNSRecord{
		txt("rec_old", "protonmail-verification=abc123"),
		txt("rec_other", "google-site-verification=xyz"),
	}
	intent := cloudflareDNSApplyIntent{
		Target:     "creatumundo.mx",
		RecordType: "TXT",
		Content:    "protonmail-verification=NEWTOKEN",
		Replace:    true,
	}

	plan := planCloudflareDNSApply(intent, live)

	if plan.Mutation != "update" {
		t.Fatalf("replace must overwrite, got %q", plan.Mutation)
	}
	if plan.Match == nil || plan.Match.ID != "rec_old" {
		t.Fatalf("replace must name the record it overwrites, got %+v", plan.Match)
	}
	if len(plan.Warnings) < 2 {
		t.Fatalf("replace must warn about the overwrite AND the untouched siblings, got %v", plan.Warnings)
	}
}

// Replace is strict for the same reason allow_pending_zone is: it is the flag
// that re-enables record destruction, so a truthy typo must not arm it.
func TestReplaceArgIsStrict(t *testing.T) {
	cases := map[string]bool{
		"true": true, "TRUE": true, " true ": true,
		"": false, "false": false, "1": false, "yes": false, "on": false, "truthy": false,
	}
	for input, want := range cases {
		req := operatorOperationRequest{Args: map[string]string{"replace": input}}
		if got := cloudflareDNSApplyReplace(req); got != want {
			t.Fatalf("replace=%q parsed as %v, want %v", input, got, want)
		}
	}
}

// The "10 mail.example.com" content form is what every runbook and every
// operator's fingers currently produce. It must keep working, and the number
// must NOT be shipped to Cloudflare as part of the MX content.
func TestMXPriorityParsedFromContentForBackCompat(t *testing.T) {
	req := operatorOperationRequest{Args: map[string]string{
		"type":    "MX",
		"content": "10 mail.protonmail.ch",
	}}
	intent := cloudflareDNSApplyIntentFromRequest(req, "tunnel.example.com")

	if intent.Content != "mail.protonmail.ch" {
		t.Fatalf("the priority must be split out of content, got content=%q", intent.Content)
	}
	if intent.Priority == nil || *intent.Priority != 10 {
		t.Fatalf("priority 10 must be parsed from content, got %v", intent.Priority)
	}
	if !intent.PriorityFromContent {
		t.Fatal("the response must be able to say the priority came from content")
	}
}

// The explicit flag wins, and the content is then used verbatim.
func TestMXPriorityFlagBeatsContent(t *testing.T) {
	req := operatorOperationRequest{Args: map[string]string{
		"type":     "MX",
		"content":  "mailsec.protonmail.ch",
		"priority": "20",
	}}
	intent := cloudflareDNSApplyIntentFromRequest(req, "tunnel.example.com")

	if intent.Priority == nil || *intent.Priority != 20 {
		t.Fatalf("explicit priority must win, got %v", intent.Priority)
	}
	if intent.Content != "mailsec.protonmail.ch" {
		t.Fatalf("content must be untouched when priority is explicit, got %q", intent.Content)
	}
	if intent.PriorityFromContent {
		t.Fatal("an explicit priority did not come from content")
	}
}

// A leading number on a type that has no priority is content, not a
// preference — "10 something" as TXT means the literal string.
func TestPriorityNotParsedForNonPriorityTypes(t *testing.T) {
	req := operatorOperationRequest{Args: map[string]string{
		"type":    "TXT",
		"content": "10 is not a priority here",
	}}
	intent := cloudflareDNSApplyIntentFromRequest(req, "tunnel.example.com")

	if intent.Content != "10 is not a priority here" {
		t.Fatalf("TXT content must never be split, got %q", intent.Content)
	}
	if intent.Priority != nil {
		t.Fatalf("TXT has no priority, got %v", intent.Priority)
	}
}

// A hostname that merely begins with digits is not a priority.
func TestPriorityNotParsedWhenLeadingTokenIsNotANumber(t *testing.T) {
	content, priority, ok := cloudflareDNSSplitPriority("MX", "mail.protonmail.ch")
	if ok {
		t.Fatalf("a bare hostname has no priority, got %d", priority)
	}
	if content != "mail.protonmail.ch" {
		t.Fatalf("content must survive unchanged, got %q", content)
	}
}

// A mistyped priority must be refused, not silently defaulted: a wrong MX
// preference that looks deliberate is worse than an error.
func TestBadPriorityIsRejectedNotDefaulted(t *testing.T) {
	for _, bad := range []string{"abc", "-1", "70000", "10.5"} {
		req := operatorOperationRequest{Args: map[string]string{"type": "MX", "priority": bad, "content": "mail.example.com"}}
		intent := cloudflareDNSApplyIntentFromRequest(req, "tunnel.example.com")
		if intent.ParseError == "" {
			t.Fatalf("priority %q must be rejected, got priority=%v", bad, intent.Priority)
		}
	}
	// 0 is a legal MX preference and must survive.
	req := operatorOperationRequest{Args: map[string]string{"type": "MX", "priority": "0", "content": "mail.example.com"}}
	intent := cloudflareDNSApplyIntentFromRequest(req, "tunnel.example.com")
	if intent.ParseError != "" {
		t.Fatalf("priority 0 is legal, got error %q", intent.ParseError)
	}
	if intent.Priority == nil || *intent.Priority != 0 {
		t.Fatalf("priority 0 must be preserved as 0, not treated as unset, got %v", intent.Priority)
	}
}

// An MX whose priority differs is a different record, so re-applying with a
// new preference must not read as a noop.
func TestPlanMXPriorityChangeIsNotNoop(t *testing.T) {
	live := []cloudflare.DNSRecord{mx("rec_primary", "mail.protonmail.ch", 10)}
	intent := cloudflareDNSApplyIntent{
		Target: "creatumundo.mx", RecordType: "MX",
		Content: "mail.protonmail.ch", Priority: intPtr(20),
	}
	if plan := planCloudflareDNSApply(intent, live); plan.Mutation == "noop" {
		t.Fatal("a changed MX preference is not a noop")
	}
}

// The error path from #530: a record Cloudflare rejects is the caller's
// problem and must carry Cloudflare's own message at a 4xx, never an opaque
// 502 that reads as a platform outage.
func TestCloudflareRejectionIsClientErrorNotBadGateway(t *testing.T) {
	rejected := &cloudflare.APIError{Code: 9007, Message: "Content for a MX record must be a valid hostname"}
	if got := cloudflareDNSApplyStatusForError(rejected); got != http.StatusUnprocessableEntity {
		t.Fatalf("a rejected record must be 422, got %d", got)
	}

	conflict := &cloudflare.APIError{Code: 81057, Message: "Record already exists."}
	if got := cloudflareDNSApplyStatusForError(conflict); got != http.StatusConflict {
		t.Fatalf("a duplicate record must be 409, got %d", got)
	}

	auth := &cloudflare.APIError{Code: 10000, Message: "Authentication error"}
	if got := cloudflareDNSApplyStatusForError(auth); got != http.StatusFailedDependency {
		t.Fatalf("a credential problem must be 424, got %d", got)
	}

	// A transport failure is the one case 502 still describes correctly.
	transport := errors.New("cloudflare: request failed: dial tcp: i/o timeout")
	if got := cloudflareDNSApplyStatusForError(transport); got != http.StatusBadGateway {
		t.Fatalf("an unreachable provider must stay 502, got %d", got)
	}

	// A wrapped Cloudflare error must still be recognised: the client wraps
	// every API error in a "failed to create DNS record for %s" fmt.Errorf.
	wrapped := errWrap(rejected)
	if got := cloudflareDNSApplyStatusForError(wrapped); got != http.StatusUnprocessableEntity {
		t.Fatalf("a wrapped Cloudflare rejection must still be 422, got %d", got)
	}
}

func errWrap(err error) error {
	return &wrappedErr{err}
}

type wrappedErr struct{ inner error }

func (w *wrappedErr) Error() string { return "failed to create DNS record for x: " + w.inner.Error() }
func (w *wrappedErr) Unwrap() error { return w.inner }

// The message an operator reads must be Cloudflare's, not a generic one.
func TestRejectionNextStepsMentionPriority(t *testing.T) {
	next := cloudflareDNSApplyNextForError(&cloudflare.APIError{Code: 9007, Message: "bad content"})
	if len(next) == 0 {
		t.Fatal("a rejection must offer a next step")
	}
	found := false
	for _, n := range next {
		if strings.Contains(n, "priority") {
			found = true
		}
	}
	if !found {
		t.Fatalf("MX/SRV guidance must mention priority, got %v", next)
	}
}

// Multi-value types are the ones where several records at one name is normal.
func TestMultiValueTypeClassification(t *testing.T) {
	for _, tt := range []string{"TXT", "MX", "NS", "SRV", "txt", " mx "} {
		if !cloudflareDNSTypeHoldsManyAtOneName(tt) {
			t.Fatalf("%q must be treated as multi-value", tt)
		}
	}
	for _, tt := range []string{"CNAME", "A", "AAAA", "CAA", ""} {
		if cloudflareDNSTypeHoldsManyAtOneName(tt) {
			t.Fatalf("%q must keep single-record semantics", tt)
		}
	}
}

// Cloudflare does not contract a stable ordering for a DNS record listing, so
// a plan that acts on "the first record returned" can show one record in the
// dry-run and overwrite a different one in the apply moments later. On the
// --replace path that is a silent destruction of a record the operator was
// never shown — the same failure shape as #530, just narrower. The same live
// set in any order must choose the same record.
func TestReplaceTargetIsOrderIndependent(t *testing.T) {
	a := txt("rec_a", "google-site-verification=one")
	b := txt("rec_b", "protonmail-verification=two")
	c := txt("rec_c", "atlassian-domain-verification=three")

	intent := cloudflareDNSApplyIntent{
		Target:     "creatumundo.mx",
		RecordType: "TXT",
		Content:    "protonmail-verification=ROTATED",
		Replace:    true,
	}

	orders := [][]cloudflare.DNSRecord{
		{a, b, c},
		{c, b, a},
		{b, c, a},
		{c, a, b},
	}
	for i, live := range orders {
		plan := planCloudflareDNSApply(intent, live)
		if plan.Match == nil {
			t.Fatalf("order %d: replace must name a record", i)
		}
		if plan.Match.ID != "rec_a" {
			t.Fatalf("order %d: replace chose %q — the target must not depend on listing order", i, plan.Match.ID)
		}
		if len(plan.Siblings) != 2 {
			t.Fatalf("order %d: the other two records must be reported untouched, got %d", i, len(plan.Siblings))
		}
	}
}

// The same determinism applies to a single-value type with an unexpected
// duplicate at the name: whichever record is chosen, it must be the same one
// the dry-run showed.
func TestSingleValueUpdateTargetIsOrderIndependent(t *testing.T) {
	a := cloudflare.DNSRecord{ID: "rec_a", Name: "app.example.com", Type: "CNAME", Content: "one.example.com"}
	b := cloudflare.DNSRecord{ID: "rec_b", Name: "app.example.com", Type: "CNAME", Content: "two.example.com"}
	intent := cloudflareDNSApplyIntent{Target: "app.example.com", RecordType: "CNAME", Content: "three.example.com"}

	first := planCloudflareDNSApply(intent, []cloudflare.DNSRecord{a, b})
	second := planCloudflareDNSApply(intent, []cloudflare.DNSRecord{b, a})

	if first.Match == nil || second.Match == nil {
		t.Fatal("both plans must name the record they update")
	}
	if first.Match.ID != second.Match.ID {
		t.Fatalf("update target depends on listing order: %q vs %q", first.Match.ID, second.Match.ID)
	}
}
