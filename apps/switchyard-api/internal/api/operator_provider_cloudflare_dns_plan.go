package api

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/madfam-org/enclii/apps/switchyard-api/internal/cloudflare"
)

// DNS record identity, and why name+type is not it.
//
// dns-apply used to identify the record it was about to change as
// (zone, name, type), taking the FIRST record Cloudflare returned for that
// pair. For CNAME that is sound — a name may hold exactly one CNAME. For TXT,
// MX, NS and SRV it is false by construction: an apex TXT name legitimately
// holds an SPF record AND every provider verification token at once, and every
// real mail provider ships an MX pair.
//
// The consequence was destructive and silent (enclii#530, hit on a live
// client zone 2026-09-07): applying an SPF TXT to an apex that already held a
// Proton ownership TXT was planned as `create` by the dry-run — which asked
// "does a TXT exist here?" against a stale read — and executed as `update` of
// the ownership record, which then no longer existed. Nothing in the plan or
// the result named the record that was destroyed.
//
// So for multi-value types the identity includes content: a record with the
// same name+type but different content is a DIFFERENT record, and the only
// non-destructive plan for it is `create`. Replacing one still has to be
// possible — that is what `--replace` is for, and it is an explicit,
// auditable choice that names what it is replacing.

// cloudflareDNSMultiValueTypes are the record types where several records at
// one name is the normal, correct shape rather than a conflict.
//
// A/AAAA are deliberately absent: multiple A records at a name are legal
// (round-robin), but every Enclii caller today writes exactly one, and
// changing that key would turn an ordinary "repoint this host" apply into a
// second A record serving the old address alongside the new one. That failure
// mode is worse than the one being fixed, so A/AAAA keep single-record
// update semantics until a caller actually needs round-robin.
var cloudflareDNSMultiValueTypes = map[string]bool{
	"TXT": true,
	"MX":  true,
	"NS":  true,
	"SRV": true,
}

// cloudflareDNSTypeHoldsManyAtOneName reports whether recordType routinely has
// more than one record at a single name.
func cloudflareDNSTypeHoldsManyAtOneName(recordType string) bool {
	return cloudflareDNSMultiValueTypes[strings.ToUpper(strings.TrimSpace(recordType))]
}

// cloudflareDNSPlan is the decision dns-apply makes before touching
// Cloudflare, and the exact decision it then executes. The dry-run and the
// apply MUST derive this from the same function against the same live read;
// they diverged before, and the divergence is what destroyed a record.
type cloudflareDNSPlan struct {
	// Mutation is "noop", "create", or "update".
	Mutation string
	// Match is the record the plan acts on: the exact-content match for a
	// noop, or the record an update will overwrite. nil for a create.
	Match *cloudflare.DNSRecord
	// Siblings are the other records already at this name+type that the plan
	// leaves untouched. Named in the response so an operator can see what a
	// create is joining, and what a --replace is choosing between.
	Siblings []cloudflare.DNSRecord
	// Warnings are plan-level facts an operator must not miss, chiefly the
	// destructive shape of a --replace.
	Warnings []string
}

// planCloudflareDNSApply decides the mutation for intent against the live
// records already at that name+type.
//
// Rules:
//   - An exact match (same content, same proxied, same effective priority)
//     is a noop, whichever type it is.
//   - For a multi-value type (TXT/MX/NS/SRV), a record with different content
//     is a DIFFERENT record: the plan is `create`, and the existing records
//     are untouched. Only an explicit replace turns that into an `update`.
//   - For every other type (CNAME/A/AAAA/...), an existing record at the name
//     is the record being changed: the plan is `update`, as before.
func planCloudflareDNSApply(intent cloudflareDNSApplyIntent, live []cloudflare.DNSRecord) cloudflareDNSPlan {
	if match := cloudflareDNSExactMatch(intent, live); match != nil {
		return cloudflareDNSPlan{
			Mutation: "noop",
			Match:    match,
			Siblings: cloudflareDNSExcluding(live, match),
		}
	}

	if len(live) == 0 {
		return cloudflareDNSPlan{Mutation: "create"}
	}

	if !cloudflareDNSTypeHoldsManyAtOneName(intent.RecordType) {
		// Single-value type: the record at this name IS the record to change.
		match := cloudflareDNSPickTarget(live)
		return cloudflareDNSPlan{
			Mutation: "update",
			Match:    &match,
			Siblings: cloudflareDNSExcluding(live, &match),
		}
	}

	if !intent.Replace {
		// The whole point of the fix: adding a second TXT/MX at a name adds a
		// record, it does not overwrite the one already there.
		return cloudflareDNSPlan{
			Mutation: "create",
			Siblings: live,
			Warnings: []string{fmt.Sprintf(
				"%d existing %s record(s) at %s are left in place; this apply ADDS a record. Pass replace=\"true\" to overwrite instead.",
				len(live), intent.RecordType, intent.Target,
			)},
		}
	}

	// Explicit replace: overwrite one existing record, and say loudly which
	// one and what else is at that name.
	match := cloudflareDNSPickTarget(live)
	warnings := []string{fmt.Sprintf(
		"replace=true: %s record %q at %s will be OVERWRITTEN with %q and its previous value is not recoverable through Enclii",
		intent.RecordType, match.Content, intent.Target, intent.Content,
	)}
	siblings := cloudflareDNSExcluding(live, &match)
	if len(siblings) > 0 {
		warnings = append(warnings, fmt.Sprintf(
			"%d other %s record(s) at %s are NOT touched by this replace; replace overwrites exactly one record",
			len(siblings), intent.RecordType, intent.Target,
		))
	}
	return cloudflareDNSPlan{
		Mutation: "update",
		Match:    &match,
		Siblings: siblings,
		Warnings: warnings,
	}
}

// cloudflareDNSPickTarget chooses which record a destructive mutation acts on,
// deterministically.
//
// Cloudflare does not contract a stable ordering for a DNS record listing, so
// taking `live[0]` means a dry-run can name one record and the apply moments
// later overwrite a different one. On the --replace path that is a silent
// destruction of a record the operator was never shown — the same failure
// shape as enclii#530, just narrower. Ordering by record ID makes the plan
// reproducible: the dry-run's `existingRecord` is the record the apply writes.
func cloudflareDNSPickTarget(live []cloudflare.DNSRecord) cloudflare.DNSRecord {
	target := live[0]
	for _, record := range live[1:] {
		if record.ID < target.ID {
			target = record
		}
	}
	return target
}

// cloudflareDNSExactMatch finds the live record that already expresses the
// intent exactly — the only safe basis for calling an apply a noop.
func cloudflareDNSExactMatch(intent cloudflareDNSApplyIntent, live []cloudflare.DNSRecord) *cloudflare.DNSRecord {
	for i := range live {
		record := live[i]
		if !cloudflareDNSContentEqual(intent.RecordType, record.Content, intent.Content) {
			continue
		}
		if record.Proxied != intent.Proxied {
			continue
		}
		if !cloudflareDNSPriorityEqual(intent, record) {
			continue
		}
		return &live[i]
	}
	return nil
}

// cloudflareDNSContentEqual compares record content the way DNS does.
//
// TXT content round-trips through Cloudflare with surrounding quotes that the
// operator did not type, and hostnames are case-insensitive; comparing raw
// strings makes an identical record read as different and re-applies it.
func cloudflareDNSContentEqual(recordType, live, desired string) bool {
	if strings.EqualFold(strings.TrimSpace(recordType), "TXT") {
		return cloudflareDNSUnquote(live) == cloudflareDNSUnquote(desired)
	}
	return strings.EqualFold(strings.TrimSpace(live), strings.TrimSpace(desired))
}

func cloudflareDNSUnquote(value string) string {
	return strings.Trim(strings.TrimSpace(value), `"`)
}

// cloudflareDNSPriorityEqual compares the effective priority of a live record
// with the intent's. For types with no priority both sides are absent and the
// comparison is trivially true.
func cloudflareDNSPriorityEqual(intent cloudflareDNSApplyIntent, record cloudflare.DNSRecord) bool {
	if !cloudflare.RecordTypeRequiresPriority(intent.RecordType) {
		return true
	}
	want := cloudflare.RecordTypeDefaultPriority
	if intent.Priority != nil {
		want = *intent.Priority
	}
	got := cloudflare.RecordTypeDefaultPriority
	if record.Priority != nil {
		got = *record.Priority
	}
	return want == got
}

func cloudflareDNSExcluding(live []cloudflare.DNSRecord, exclude *cloudflare.DNSRecord) []cloudflare.DNSRecord {
	if exclude == nil {
		return live
	}
	out := make([]cloudflare.DNSRecord, 0, len(live))
	for _, record := range live {
		if record.ID != "" && record.ID == exclude.ID {
			continue
		}
		out = append(out, record)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// cloudflareDNSSplitPriority pulls a leading MX preference out of a content
// string.
//
// Before a --priority flag existed, the only way to express an MX preference
// through this command was to type it into --content as `"10 mail.example.com"`
// — which every runbook and every operator's muscle memory now does. That form
// keeps working: the number is parsed off the front and becomes the priority
// field, and the remainder is the content. Sending "10 mail.example.com" to
// Cloudflare as MX content is not a valid record.
//
// Returns the remaining content, the parsed priority, and whether one was
// found. Only applies to types that actually take a priority.
func cloudflareDNSSplitPriority(recordType, content string) (string, int, bool) {
	if !cloudflare.RecordTypeRequiresPriority(recordType) {
		return content, 0, false
	}
	trimmed := strings.TrimSpace(content)
	head, rest, found := strings.Cut(trimmed, " ")
	if !found {
		return trimmed, 0, false
	}
	priority, err := strconv.Atoi(strings.TrimSpace(head))
	if err != nil || priority < 0 || priority > 65535 {
		return trimmed, 0, false
	}
	rest = strings.TrimSpace(rest)
	if rest == "" {
		return trimmed, 0, false
	}
	return rest, priority, true
}

// cloudflareDNSParsePriority reads an explicit priority argument.
//
// Returns an error rather than a silent default: a mistyped priority that
// falls back to 10 writes a wrong MX preference that looks deliberate.
func cloudflareDNSParsePriority(raw string) (*int, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return nil, nil
	}
	priority, err := strconv.Atoi(value)
	if err != nil {
		return nil, fmt.Errorf("priority %q is not a number", raw)
	}
	if priority < 0 || priority > 65535 {
		return nil, fmt.Errorf("priority %d is out of range (0-65535)", priority)
	}
	return &priority, nil
}
