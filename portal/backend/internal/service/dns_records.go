// Package service: DNS record generation from a Stalwart DNS zone file.
//
// Stalwart returns the complete authoritative zone for a domain in
// Domain.dnsZoneFile. The onboarding wizard only needs four records — MX, SPF,
// DKIM, and DMARC — so we parse those out of the zone text. Everything else
// (TLSA, SRV, CAA, autoconfig, etc.) is out of Sprint 1.3 scope.
//
// The zone format is RFC-1035 style: an owner name at the start of a line,
// optional IN class, a type, and a value. TXT values may be a single quoted
// string or, for long DKIM keys, a parenthesised block spanning multiple
// quoted strings across several lines.
package service

import (
	"bufio"
	"fmt"
	"strings"

	"github.com/jetswu/cloudsuite/portal/backend/internal/domain"
)

// GenerateDNSRecords parses a Stalwart dnsZoneFile and extracts the onboarding
// records: MX (one), SPF (TXT @), DMARC (TXT _dmarc), and DKIM (TXT
// *_domainkey, one per algorithm). The Name field is the owner name WITHOUT
// the domain suffix ("@" for the apex).
func GenerateDNSRecords(domainName string, zoneFile string) ([]domain.DNSRecord, error) {
	domainName = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(domainName), "."))
	if domainName == "" {
		return nil, fmt.Errorf("empty domain name")
	}

	records := []domain.DNSRecord{}
	seen := map[string]bool{} // dedup key: purpose + "|" + name

	scanner := bufio.NewScanner(strings.NewReader(zoneFile))
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var (
		curOwner string
		curType  string
		curTXT   strings.Builder
		curMX    *domain.DNSRecord
		inParens bool
	)

	// flush appends the accumulated record (if any) to the output.
	flush := func() {
		if curOwner == "" || curType == "" {
			return
		}
		ownerName := normalizeOwner(curOwner, domainName)

		switch curType {
		case "TXT":
			val := strings.TrimSpace(curTXT.String())
			// Collapse whitespace runs (indentation of multi-line
			// zone blocks, tabs) into single spaces so the stored
			// value is clean: "v=DKIM1; k=rsa; p=...".
			val = strings.Join(strings.Fields(val), " ")
			// Zone-file parens can leak INSIDE the quoted content
			// (Stalwart glues the closing ")" onto the last string).
			// DKIM base64, SPF, and DMARC values never legitimately
			// contain parens, so drop them.
			val = strings.ReplaceAll(val, "(", "")
			val = strings.ReplaceAll(val, ")", "")
			rec := domain.DNSRecord{RecordType: "TXT", Name: ownerName, Value: val, IsRequired: true}
			switch {
			case ownerName == "@" && strings.HasPrefix(val, "v=spf1"):
				rec.Purpose = domain.PurposeSPF
			case ownerName == "_dmarc" && strings.HasPrefix(val, "v=DMARC1"):
				rec.Purpose = domain.PurposeDMARC
			case strings.HasSuffix(ownerName, "._domainkey") && strings.HasPrefix(val, "v=DKIM1"):
				rec.Purpose = domain.PurposeDKIM
			default:
				rec = domain.DNSRecord{}
			}
			if rec.Purpose != "" && !seen[dedupKey(rec.Purpose, ownerName)] {
				records = append(records, rec)
				seen[dedupKey(rec.Purpose, ownerName)] = true
			}
		case "MX":
			if curMX != nil && curMX.Name == "@" && !seen[dedupKey(domain.PurposeMX, "@")] {
				records = append(records, *curMX)
				seen[dedupKey(domain.PurposeMX, "@")] = true
			}
		}

		curOwner, curType = "", ""
		curTXT.Reset()
		curMX = nil
	}

	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, ";") {
			continue
		}

		// Continuation of a parenthesised TXT value: append only the content
		// INSIDE the quoted strings. The line's indentation and the closing
		// ")" are zone-file formatting and must NOT leak into the value.
		if inParens {
			curTXT.WriteString(extractQuoted(line))
			if strings.Contains(line, ")") {
				inParens = false
				flush()
			}
			continue
		}

		// New record line.
		owner, rest := splitOwnerValue(trimmed)
		if owner == "" || rest == "" {
			continue
		}

		// Identify the type token (first token after IN removal).
		fields := strings.Fields(rest)
		if len(fields) == 0 {
			continue
		}
		rtype := fields[0]

		// Set current owner/type; value accumulates below.
		curOwner = owner
		curType = rtype
		curTXT.Reset()
		curMX = nil

		switch rtype {
		case "MX":
			if len(fields) >= 3 {
				prio := 0
				fmt.Sscanf(fields[1], "%d", &prio)
				value := strings.TrimSuffix(fields[2], ".")
				curMX = &domain.DNSRecord{
					RecordType: "MX",
					Name:       normalizeOwner(owner, domainName),
					Value:      value,
					Priority:   &prio,
					Purpose:    domain.PurposeMX,
					IsRequired: true,
				}
			}
			flush()

		case "TXT":
			// Extract quoted strings from the rest of THIS line.
			curTXT.WriteString(extractQuoted(rest))
			if strings.Contains(rest, "(") && !strings.Contains(rest, ")") {
				inParens = true
			} else {
				flush()
			}

		default:
			// SRV/TLSA/CAA/etc. — out of scope. Skip.
			curOwner, curType = "", ""
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan zone file: %w", err)
	}
	// A dangling multi-line record without a closing ")".
	if inParens {
		flush()
	}
	return records, nil
}

func dedupKey(purpose domain.DNSRecordPurpose, name string) string {
	return string(purpose) + "|" + name
}

// splitOwnerValue splits a zone line into its owner name and the remainder
// (type + value). Handles the optional "IN" class token.
func splitOwnerValue(line string) (string, string) {
	fields := strings.Fields(line)
	if len(fields) < 2 {
		return "", ""
	}
	owner := fields[0]
	if strings.EqualFold(fields[1], "IN") {
		if len(fields) < 3 {
			return "", ""
		}
		return owner, strings.Join(fields[2:], " ")
	}
	return owner, strings.Join(fields[1:], " ")
}

// normalizeOwner strips the domain suffix and trailing dot so the owner
// becomes a short name relative to the domain ("@" for the apex).
func normalizeOwner(owner, domainName string) string {
	owner = strings.ToLower(owner)
	owner = strings.TrimSuffix(owner, ".")
	if owner == domainName {
		return "@"
	}
	owner = strings.TrimSuffix(owner, "."+domainName)
	if owner == "" {
		return "@"
	}
	return owner
}

// extractQuoted returns the concatenation of all double-quoted substrings in
// s, preserving the content inside the quotes (spaces included).
func extractQuoted(s string) string {
	var b strings.Builder
	inQuote := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '"' {
			inQuote = !inQuote
			continue
		}
		if inQuote {
			b.WriteByte(c)
		}
	}
	return b.String()
}


