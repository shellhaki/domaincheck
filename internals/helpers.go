package internals

import (
	"errors"
	"fmt"
	"net"
	"regexp"
	"strings"
)

var labelPattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$`)

func NormalizeDomain(domain string) (string, error) {
	domain = strings.TrimSpace(strings.TrimSuffix(strings.ToLower(domain), "."))
	if domain == "" || len(domain) > 253 || strings.Contains(domain, "://") || net.ParseIP(domain) != nil {
		return "", errors.New("invalid domain name")
	}
	parts := strings.Split(domain, ".")
	if len(parts) < 2 {
		return "", errors.New("domain must include a TLD")
	}
	for _, part := range parts {
		if !labelPattern.MatchString(part) {
			return "", errors.New("invalid domain label")
		}
	}
	return domain, nil
}

func DomainTLD(domain string) string {
	parts := strings.Split(domain, ".")
	return parts[len(parts)-1]
}

func RDAPDomainURL(base, domain string) string {
	return fmt.Sprintf("%sdomain/%s", strings.TrimSuffix(base, "/")+"/", domain)
}
