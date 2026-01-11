package cloudflare

import (
	"context"
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

// GetDNSRecord retrieves a specific DNS record by domain name
func (c *Client) GetDNSRecord(ctx context.Context, domain string) (*DNSRecord, error) {
	query := url.Values{}
	query.Set("name", domain)

	var resp APIResponse[[]DNSRecord]
	path := fmt.Sprintf("/zones/%s/dns_records", c.zoneID)

	if err := c.get(ctx, path, query, &resp); err != nil {
		return nil, fmt.Errorf("failed to get DNS record for %s: %w", domain, err)
	}

	if !resp.Success || len(resp.Result) == 0 {
		return nil, nil // Record not found
	}

	return &resp.Result[0], nil
}

// GetDNSRecordByType retrieves a DNS record by name and type
func (c *Client) GetDNSRecordByType(ctx context.Context, domain, recordType string) (*DNSRecord, error) {
	query := url.Values{}
	query.Set("name", domain)
	query.Set("type", recordType)

	var resp APIResponse[[]DNSRecord]
	path := fmt.Sprintf("/zones/%s/dns_records", c.zoneID)

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
