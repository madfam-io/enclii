package cloudflare

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/sirupsen/logrus"
)

// ListDNSRecords retrieves all DNS records for the configured zone
func (c *Client) ListDNSRecords(ctx context.Context) ([]DNSRecord, error) {
	return c.ListDNSRecordsForZone(ctx, c.zoneID)
}

// ListDNSRecordsForZone retrieves all DNS records for a specific zone
func (c *Client) ListDNSRecordsForZone(ctx context.Context, zoneID string) ([]DNSRecord, error) {
	var allRecords []DNSRecord
	page := 1
	perPage := 100

	for {
		query := url.Values{}
		query.Set("page", fmt.Sprintf("%d", page))
		query.Set("per_page", fmt.Sprintf("%d", perPage))

		var resp APIResponse[[]DNSRecord]
		path := fmt.Sprintf("/zones/%s/dns_records", zoneID)

		if err := c.get(ctx, path, query, &resp); err != nil {
			return nil, fmt.Errorf("failed to list DNS records: %w", err)
		}

		if !resp.Success {
			if len(resp.Errors) > 0 {
				return nil, fmt.Errorf("API error: %s", resp.Errors[0].Message)
			}
			return nil, fmt.Errorf("unknown API error")
		}

		allRecords = append(allRecords, resp.Result...)

		if resp.ResultInfo == nil || page >= resp.ResultInfo.TotalPages {
			break
		}
		page++
	}

	logrus.WithField("count", len(allRecords)).Debug("Retrieved DNS records from Cloudflare")
	return allRecords, nil
}

// GetDNSRecord retrieves a specific DNS record by domain name.
// Uses FindZoneForDomain to support multi-zone lookups (e.g., madfam.io, tezca.mx).
func (c *Client) GetDNSRecord(ctx context.Context, domain string) (*DNSRecord, error) {
	zone, err := c.FindZoneForDomain(ctx, domain)
	if err != nil {
		return nil, fmt.Errorf("failed to find zone for %s: %w", domain, err)
	}

	query := url.Values{}
	query.Set("name", domain)

	var resp APIResponse[[]DNSRecord]
	path := fmt.Sprintf("/zones/%s/dns_records", zone.ID)

	if err := c.get(ctx, path, query, &resp); err != nil {
		return nil, fmt.Errorf("failed to get DNS record for %s: %w", domain, err)
	}

	if !resp.Success || len(resp.Result) == 0 {
		return nil, nil // Record not found
	}

	return &resp.Result[0], nil
}

// GetDNSRecordByType retrieves a DNS record by name and type.
// Uses FindZoneForDomain to support multi-zone lookups.
func (c *Client) GetDNSRecordByType(ctx context.Context, domain, recordType string) (*DNSRecord, error) {
	zone, err := c.FindZoneForDomain(ctx, domain)
	if err != nil {
		return nil, fmt.Errorf("failed to find zone for %s: %w", domain, err)
	}

	query := url.Values{}
	query.Set("name", domain)
	query.Set("type", recordType)

	var resp APIResponse[[]DNSRecord]
	path := fmt.Sprintf("/zones/%s/dns_records", zone.ID)

	if err := c.get(ctx, path, query, &resp); err != nil {
		return nil, fmt.Errorf("failed to get %s record for %s: %w", recordType, domain, err)
	}

	if !resp.Success || len(resp.Result) == 0 {
		return nil, nil
	}

	return &resp.Result[0], nil
}

// VerifyDomainDNS checks if a domain's DNS is correctly configured to point to the tunnel
func (c *Client) VerifyDomainDNS(ctx context.Context, domain, expectedCNAME string) (*DNSVerificationResult, error) {
	result := &DNSVerificationResult{
		Domain:          domain,
		ExpectedContent: expectedCNAME,
	}

	// First, try to find a CNAME record
	record, err := c.GetDNSRecordByType(ctx, domain, "CNAME")
	if err != nil {
		return nil, fmt.Errorf("failed to verify DNS for %s: %w", domain, err)
	}

	if record != nil {
		result.RecordExists = true
		result.RecordType = "CNAME"
		result.RecordContent = record.Content
		result.Proxied = record.Proxied
		result.IsCorrect = strings.EqualFold(record.Content, expectedCNAME)
		return result, nil
	}

	// If no CNAME, check for A record (proxied domains might use A records)
	record, err = c.GetDNSRecordByType(ctx, domain, "A")
	if err != nil {
		return nil, fmt.Errorf("failed to verify DNS for %s: %w", domain, err)
	}

	if record != nil {
		result.RecordExists = true
		result.RecordType = "A"
		result.RecordContent = record.Content
		result.Proxied = record.Proxied
		// A records with proxied enabled might still be correctly configured
		result.IsCorrect = record.Proxied
		return result, nil
	}

	// No record found
	result.RecordExists = false
	result.IsCorrect = false
	return result, nil
}

// VerifyDomainTXTRecord checks for a specific TXT verification record
func (c *Client) VerifyDomainTXTRecord(ctx context.Context, domain, expectedValue string) (bool, error) {
	record, err := c.GetDNSRecordByType(ctx, domain, "TXT")
	if err != nil {
		return false, fmt.Errorf("failed to verify TXT record for %s: %w", domain, err)
	}

	if record == nil {
		return false, nil
	}

	return strings.Contains(record.Content, expectedValue), nil
}

// CheckDomainExists checks if a domain has any DNS records in the zone
func (c *Client) CheckDomainExists(ctx context.Context, domain string) (bool, error) {
	record, err := c.GetDNSRecord(ctx, domain)
	if err != nil {
		return false, err
	}
	return record != nil, nil
}

// CreateDNSRecord creates a new DNS record in the configured zone
func (c *Client) CreateDNSRecord(ctx context.Context, name, recordType, content string, proxied bool) (*DNSRecord, error) {
	return c.CreateDNSRecordInZone(ctx, c.zoneID, name, recordType, content, proxied)
}

// CreateDNSRecordInZone creates a new DNS record in a specific zone.
//
// It carries no explicit priority, so an MX/SRV create through this entry
// point falls back to RecordTypeDefaultPriority — Cloudflare rejects an MX
// create with no priority field at all (HTTP 400), and that rejection used to
// surface as an opaque 5xx.
func (c *Client) CreateDNSRecordInZone(ctx context.Context, zoneID, name, recordType, content string, proxied bool) (*DNSRecord, error) {
	return c.createDNSRecord(ctx, zoneID, name, recordType, content, proxied, nil, "Managed by Enclii platform")
}

// CreateDNSRecordInZoneWithPriority creates a DNS record, including MX/SRV
// priority.
//
// priority < 0 means "not specified"; 0 is a legal MX preference and is sent
// verbatim. Callers that genuinely have no priority for an MX get the
// documented default rather than a Cloudflare 400.
func (c *Client) CreateDNSRecordInZoneWithPriority(ctx context.Context, zoneID, name, recordType, content string, proxied bool, priority int) (*DNSRecord, error) {
	var want *int
	if priority >= 0 {
		want = &priority
	}
	return c.createDNSRecord(ctx, zoneID, name, recordType, content, proxied, want, "Managed by Enclii platform (Resend DNS)")
}

// RecordTypeDefaultPriority is the preference used when an MX/SRV record is
// created without one. Cloudflare requires the field for those types and
// answers HTTP 400 without it; 10 is the conventional single-host MX
// preference, and a wrong-but-present preference is a record an operator can
// see and correct, where a 400 rendered as a 5xx is neither.
const RecordTypeDefaultPriority = 10

// RecordTypeRequiresPriority reports whether Cloudflare requires a priority
// field for recordType.
func RecordTypeRequiresPriority(recordType string) bool {
	switch strings.ToUpper(strings.TrimSpace(recordType)) {
	case "MX", "SRV", "URI":
		return true
	default:
		return false
	}
}

func (c *Client) createDNSRecord(ctx context.Context, zoneID, name, recordType, content string, proxied bool, priority *int, comment string) (*DNSRecord, error) {
	payload := map[string]any{
		"type":    recordType,
		"name":    name,
		"content": content,
		"proxied": proxied,
		"ttl":     1, // Auto
		"comment": comment,
	}
	if p := effectivePriority(recordType, priority); p != nil {
		payload["priority"] = *p
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal DNS record: %w", err)
	}

	var resp APIResponse[DNSRecord]
	path := fmt.Sprintf("/zones/%s/dns_records", zoneID)

	if err := c.post(ctx, path, bytes.NewReader(payloadBytes), &resp); err != nil {
		return nil, fmt.Errorf("failed to create DNS record for %s: %w", name, err)
	}

	if !resp.Success {
		if len(resp.Errors) > 0 {
			return nil, fmt.Errorf("API error creating DNS record: %s", resp.Errors[0].Message)
		}
		return nil, fmt.Errorf("unknown API error creating DNS record")
	}

	logrus.WithFields(logrus.Fields{
		"name":    name,
		"type":    recordType,
		"content": content,
		"proxied": proxied,
	}).Info("Created DNS record in Cloudflare")

	return &resp.Result, nil
}

// effectivePriority resolves the priority to send for a record type: nil for
// types that have none, the caller's value when given, and the documented
// default for a priority-requiring type the caller left unset.
func effectivePriority(recordType string, priority *int) *int {
	if !RecordTypeRequiresPriority(recordType) {
		return nil
	}
	if priority != nil {
		return priority
	}
	fallback := RecordTypeDefaultPriority
	return &fallback
}

// UpdateDNSRecordInZone updates an existing DNS record in a specific zone,
// preserving the record's current priority.
func (c *Client) UpdateDNSRecordInZone(ctx context.Context, zoneID string, record DNSRecord, content string, proxied bool) (*DNSRecord, error) {
	return c.UpdateDNSRecordInZoneWithPriority(ctx, zoneID, record, content, proxied, record.Priority)
}

// UpdateDNSRecordInZoneWithPriority updates a record and sets its MX/SRV
// priority. A nil priority on a priority-requiring type keeps the record's
// own value, falling back to RecordTypeDefaultPriority — Cloudflare's PUT
// replaces the whole record, so an MX update that omits priority is a 400,
// not a partial edit.
func (c *Client) UpdateDNSRecordInZoneWithPriority(ctx context.Context, zoneID string, record DNSRecord, content string, proxied bool, priority *int) (*DNSRecord, error) {
	if record.ID == "" {
		return nil, fmt.Errorf("record ID is required")
	}
	if record.Name == "" {
		return nil, fmt.Errorf("record name is required")
	}

	recordType := record.Type
	if recordType == "" {
		recordType = "CNAME"
	}
	ttl := record.TTL
	if ttl == 0 {
		ttl = 1
	}

	if priority == nil {
		priority = record.Priority
	}

	payload := map[string]any{
		"type":    recordType,
		"name":    record.Name,
		"content": content,
		"proxied": proxied,
		"ttl":     ttl,
		"comment": "Managed by Enclii platform",
	}
	if p := effectivePriority(recordType, priority); p != nil {
		payload["priority"] = *p
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal DNS record update: %w", err)
	}

	var resp APIResponse[DNSRecord]
	path := fmt.Sprintf("/zones/%s/dns_records/%s", zoneID, record.ID)

	if err := c.put(ctx, path, bytes.NewReader(payloadBytes), &resp); err != nil {
		return nil, fmt.Errorf("failed to update DNS record for %s: %w", record.Name, err)
	}

	if !resp.Success {
		if len(resp.Errors) > 0 {
			return nil, fmt.Errorf("API error updating DNS record: %s", resp.Errors[0].Message)
		}
		return nil, fmt.Errorf("unknown API error updating DNS record")
	}

	logrus.WithFields(logrus.Fields{
		"name":    record.Name,
		"type":    recordType,
		"content": content,
		"proxied": proxied,
	}).Info("Updated DNS record in Cloudflare")

	return &resp.Result, nil
}

// DeleteDNSRecord deletes a DNS record by its ID in the configured zone
func (c *Client) DeleteDNSRecord(ctx context.Context, recordID string) error {
	return c.DeleteDNSRecordInZone(ctx, c.zoneID, recordID)
}

// DeleteDNSRecordInZone deletes a DNS record by its ID in a specific zone
func (c *Client) DeleteDNSRecordInZone(ctx context.Context, zoneID, recordID string) error {
	var resp APIResponse[struct {
		ID string `json:"id"`
	}]
	path := fmt.Sprintf("/zones/%s/dns_records/%s", zoneID, recordID)

	if err := c.httpDelete(ctx, path, &resp); err != nil {
		return fmt.Errorf("failed to delete DNS record %s: %w", recordID, err)
	}

	if !resp.Success {
		if len(resp.Errors) > 0 {
			return fmt.Errorf("API error deleting DNS record: %s", resp.Errors[0].Message)
		}
		return fmt.Errorf("unknown API error deleting DNS record")
	}

	logrus.WithField("record_id", recordID).Info("Deleted DNS record from Cloudflare")
	return nil
}

// ListAccountZones lists all zones accessible by the API token (any status).
func (c *Client) ListAccountZones(ctx context.Context) ([]Zone, error) {
	return c.listZonesPaginated(ctx, url.Values{})
}

// ListZones lists active zones accessible by the API token.
func (c *Client) ListZones(ctx context.Context) ([]Zone, error) {
	query := url.Values{}
	query.Set("status", "active")
	return c.listZonesPaginated(ctx, query)
}

func (c *Client) listZonesPaginated(ctx context.Context, baseQuery url.Values) ([]Zone, error) {
	var allZones []Zone
	page := 1
	perPage := 50

	for {
		query := url.Values{}
		for k, v := range baseQuery {
			for _, item := range v {
				query.Add(k, item)
			}
		}
		query.Set("page", fmt.Sprintf("%d", page))
		query.Set("per_page", fmt.Sprintf("%d", perPage))

		var resp APIResponse[[]Zone]
		if err := c.get(ctx, "/zones", query, &resp); err != nil {
			return nil, fmt.Errorf("failed to list zones: %w", err)
		}

		if !resp.Success {
			if len(resp.Errors) > 0 {
				return nil, fmt.Errorf("API error: %s", resp.Errors[0].Message)
			}
			return nil, fmt.Errorf("unknown API error")
		}

		allZones = append(allZones, resp.Result...)

		done, err := lastPage(resp.ResultInfo, page, perPage, len(resp.Result), "zone listing")
		if err != nil {
			return nil, err
		}
		if done {
			break
		}
		page++
	}

	return allZones, nil
}

// lastPage reports whether a paginated Cloudflare listing is complete after the
// page just read, and refuses to guess when it cannot tell.
//
// A missing result_info used to break the loop and return the pages collected
// so far with a NIL error. For zones that is the worst possible shape: a
// truncated listing is indistinguishable from a complete one, so
// FindZoneForDomain concludes ErrZoneNotFound, and callers read that sentinel
// as Cloudflare's positive confirmation that the domain is client-owned. A
// silent truncation became a confirmed miss.
//
// A short page is still provably the last one — Cloudflare fills a page before
// starting another — so the common stub/response shape with no result_info and
// fewer than per_page results is accepted. Only a FULL page with no pagination
// metadata is undecidable, and that is refused.
func lastPage(info *ResultInfo, page, perPage, got int, what string) (bool, error) {
	if info != nil {
		return page >= info.TotalPages, nil
	}
	if got < perPage {
		return true, nil
	}
	return false, fmt.Errorf(
		"cloudflare: %s returned a full page (%d results) with no result_info, so whether more pages exist is unknown; "+
			"refusing to report a possibly truncated listing as complete", what, got)
}

// ErrZoneNotFound is returned by FindZoneForDomain when the account genuinely
// holds no zone for the domain. It is deliberately the ONLY signal for that
// case: every transport, HTTP, authentication or pagination failure is
// returned as-is so callers can tell "the zone is not ours" apart from "we
// could not find out". Callers that branch on the answer — the domain
// provisioning mechanism decision, zone auto-creation — must fail closed on
// anything that is not this sentinel.
var ErrZoneNotFound = errors.New("cloudflare: no zone found for domain")

// ZoneNotActiveError reports that the account does hold a zone for the domain
// but Cloudflare is not serving it (status "pending", "moved", "initializing",
// ...). ListZones filters status=active, so such a zone is absent from the
// normal listing; treating that absence as ErrZoneNotFound would silently move
// an existing domain onto a different provisioning mechanism.
type ZoneNotActiveError struct {
	Domain   string
	ZoneName string
	Status   string
}

func (e *ZoneNotActiveError) Error() string {
	return fmt.Sprintf("cloudflare: zone %s for domain %s is not active (status %q)",
		e.ZoneName, e.Domain, e.Status)
}

// FindZoneForDomain finds the Cloudflare zone that manages a given domain
// For example, "api.qubic.quest" would match zone "qubic.quest"
//
// Errors are three distinguishable kinds:
//   - ErrZoneNotFound     — the account holds no zone for this domain
//   - *ZoneNotActiveError — a zone exists but Cloudflare is not serving it
//   - anything else       — the lookup failed and the answer is unknown
func (c *Client) FindZoneForDomain(ctx context.Context, domain string) (*Zone, error) {
	zones, err := c.ListZones(ctx)
	if err != nil {
		// Transport / HTTP / auth / pagination failure. Returned verbatim so
		// it is never mistaken for "not found".
		return nil, err
	}

	if match := bestZoneMatch(zones, domain); match != nil {
		return match, nil
	}

	// ListZones filters status=active. A zone that is pending activation or
	// has moved therefore reads as absent, so ask again without the filter
	// before concluding the account does not hold it.
	allZones, allErr := c.ListAccountZones(ctx)
	if allErr != nil {
		return nil, fmt.Errorf("no active Cloudflare zone matched %s and the unfiltered zone listing failed: %w", domain, allErr)
	}
	if match := bestZoneMatch(allZones, domain); match != nil {
		return nil, &ZoneNotActiveError{Domain: domain, ZoneName: match.Name, Status: match.Status}
	}

	return nil, fmt.Errorf("%w: %s", ErrZoneNotFound, domain)
}

// FindZoneForDomainIncludingPending resolves the account's zone for domain,
// additionally accepting zones Cloudflare is not serving yet (typically status
// "pending": created in the account but not yet delegated at the registrar).
//
// Records written into a pending zone are inert until the registrar delegates
// to the zone's nameservers — which is exactly what pre-cutover staging needs:
// seed the zone while the old DNS provider still serves, so the later
// delegation flip changes nothing visible. Callers MUST surface the zone's
// status to the operator; a pending zone answering "write succeeded" without
// that context reads as live DNS when it is not.
//
// Failure modes mirror FindZoneForDomain: ErrZoneNotFound when the account
// holds nothing for the domain; transport/HTTP errors returned verbatim.
func (c *Client) FindZoneForDomainIncludingPending(ctx context.Context, domain string) (*Zone, error) {
	zone, err := c.FindZoneForDomain(ctx, domain)
	if err == nil {
		return zone, nil
	}
	var notActive *ZoneNotActiveError
	if !errors.As(err, &notActive) {
		return nil, err
	}
	allZones, allErr := c.ListAccountZones(ctx)
	if allErr != nil {
		return nil, fmt.Errorf("zone %s is not active and re-resolving it from the unfiltered listing failed: %w", notActive.ZoneName, allErr)
	}
	if match := bestZoneMatch(allZones, domain); match != nil {
		return match, nil
	}
	return nil, fmt.Errorf("%w: %s", ErrZoneNotFound, domain)
}

// GetDNSRecordByTypeInZone is GetDNSRecordByType for callers that already
// resolved the zone — possibly leniently, via
// FindZoneForDomainIncludingPending — and must not have the read re-run the
// strict active-only zone lookup.
func (c *Client) GetDNSRecordByTypeInZone(ctx context.Context, zoneID, domain, recordType string) (*DNSRecord, error) {
	records, err := c.ListDNSRecordsByTypeInZone(ctx, zoneID, domain, recordType)
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return nil, nil
	}
	return &records[0], nil
}

// ListDNSRecordsByTypeInZone returns EVERY record at (zone, name, type), not
// just the first.
//
// GetDNSRecordByTypeInZone answers `resp.Result[0]` and discards the rest,
// which silently asserts that a name holds at most one record of a type. For
// TXT, MX, NS and SRV that assertion is false by design — SPF plus provider
// verification tokens share the apex TXT name, and every real mail provider
// ships an MX pair. A caller that plans a mutation from the first record alone
// will overwrite one of a set it never saw (enclii#530: an SPF TXT applied as
// an "update" destroyed a Proton ownership TXT on a live zone).
func (c *Client) ListDNSRecordsByTypeInZone(ctx context.Context, zoneID, domain, recordType string) ([]DNSRecord, error) {
	query := url.Values{}
	query.Set("name", domain)
	query.Set("type", recordType)
	query.Set("per_page", "100")

	var resp APIResponse[[]DNSRecord]
	path := fmt.Sprintf("/zones/%s/dns_records", zoneID)

	if err := c.get(ctx, path, query, &resp); err != nil {
		return nil, fmt.Errorf("failed to get %s records for %s: %w", recordType, domain, err)
	}

	if !resp.Success {
		return nil, nil
	}

	return resp.Result, nil
}

// bestZoneMatch returns the most specific zone covering domain (longest suffix
// match), or nil when none does.
//
// The comparison is case-insensitive. DNS is case-insensitive and Cloudflare
// returns zone names lowercased, so a case-exact match would answer "we hold no
// zone for api.Madfam.io" — and the caller reads that answer as "this domain
// belongs to a client", which reroutes a live MADFAM hostname onto the
// Cloudflare for SaaS path with no Cloudflare failure involved. Callers
// canonicalise too; this is the second lock on the same door.
func bestZoneMatch(zones []Zone, domain string) *Zone {
	var bestMatch *Zone
	bestLen := 0

	needle := strings.ToLower(strings.TrimSpace(domain))
	for i, zone := range zones {
		if zone.Name == "" {
			continue
		}
		zoneName := strings.ToLower(zone.Name)
		if needle == zoneName || strings.HasSuffix(needle, "."+zoneName) {
			if len(zoneName) > bestLen {
				bestMatch = &zones[i]
				bestLen = len(zoneName)
			}
		}
	}

	return bestMatch
}

// CreateZone creates a new Cloudflare zone for a domain.
// Uses jump_start to auto-scan for existing DNS records.
func (c *Client) CreateZone(ctx context.Context, name string) (*Zone, error) {
	payload := struct {
		Name    string `json:"name"`
		Account struct {
			ID string `json:"id"`
		} `json:"account"`
		JumpStart bool `json:"jump_start"`
	}{
		Name:      name,
		JumpStart: true,
	}
	payload.Account.ID = c.accountID

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal zone creation request: %w", err)
	}

	var resp APIResponse[Zone]
	if err := c.post(ctx, "/zones", bytes.NewReader(payloadBytes), &resp); err != nil {
		return nil, fmt.Errorf("failed to create zone %s: %w", name, err)
	}

	if !resp.Success {
		if len(resp.Errors) > 0 {
			return nil, fmt.Errorf("API error creating zone %s: %s", name, resp.Errors[0].Message)
		}
		return nil, fmt.Errorf("unknown API error creating zone %s", name)
	}

	logrus.WithFields(logrus.Fields{
		"zone_id":      resp.Result.ID,
		"zone_name":    resp.Result.Name,
		"status":       resp.Result.Status,
		"name_servers": resp.Result.NameServers,
	}).Info("Created Cloudflare zone")

	return &resp.Result, nil
}

// EnsureZoneForDomain finds the Cloudflare zone for a domain, creating it if missing.
// Extracts the apex domain (last 2 segments) from the FQDN for zone creation.
//
// A zone is created ONLY when Cloudflare confirmed the account does not hold
// one (ErrZoneNotFound). A failed lookup, or a zone that exists but is not
// active, is returned as-is: creating a zone because a listing timed out would
// take over a domain we were never asked to take over.
func (c *Client) EnsureZoneForDomain(ctx context.Context, domain string) (*Zone, error) {
	zone, err := c.FindZoneForDomain(ctx, domain)
	if err == nil {
		return zone, nil
	}
	if !errors.Is(err, ErrZoneNotFound) {
		return nil, err
	}

	// Extract apex domain: "api.tezca.mx" → "tezca.mx"
	parts := strings.Split(domain, ".")
	if len(parts) < 2 {
		return nil, fmt.Errorf("cannot extract apex domain from %s", domain)
	}
	apex := strings.Join(parts[len(parts)-2:], ".")

	logrus.WithFields(logrus.Fields{
		"domain": domain,
		"apex":   apex,
	}).Info("Zone not found, creating new Cloudflare zone")

	return c.CreateZone(ctx, apex)
}

// EnsureDNSRecord creates a CNAME record for the domain if it doesn't already exist.
// If a record exists with different content, it is left unchanged.
// Returns the record and whether it was created.
func (c *Client) EnsureDNSRecord(ctx context.Context, domain, cnameTarget string) (*DNSRecord, bool, error) {
	// Find which zone manages this domain
	zone, err := c.FindZoneForDomain(ctx, domain)
	if err != nil {
		return nil, false, err
	}

	// Check if record already exists in that zone
	query := url.Values{}
	query.Set("name", domain)
	query.Set("type", "CNAME")

	var resp APIResponse[[]DNSRecord]
	path := fmt.Sprintf("/zones/%s/dns_records", zone.ID)

	if err := c.get(ctx, path, query, &resp); err != nil {
		return nil, false, fmt.Errorf("failed to check DNS record for %s: %w", domain, err)
	}

	if resp.Success && len(resp.Result) > 0 {
		record := resp.Result[0]
		if !strings.EqualFold(record.Content, cnameTarget) || !record.Proxied {
			updated, err := c.UpdateDNSRecordInZone(ctx, zone.ID, record, cnameTarget, true)
			if err != nil {
				return nil, false, err
			}
			return updated, false, nil
		}

		return &record, false, nil
	}

	// Create the record
	record, err := c.CreateDNSRecordInZone(ctx, zone.ID, domain, "CNAME", cnameTarget, true)
	if err != nil {
		return nil, false, err
	}

	return record, true, nil
}
