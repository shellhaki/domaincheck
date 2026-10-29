package internals

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"shellhaki/domaincheck/models"
)

var rdapHTTPClient = &http.Client{Timeout: 8 * time.Second}

func Lookup(ctx context.Context, domain string, registry *Registry) (models.DomainResult, error) {
	result := models.DomainResult{Domain: domain}
	servers := registry.Servers(DomainTLD(domain))
	if len(servers) == 0 {
		result.Status = "unknown"
		result.Providers = CheckProviders(ctx, domain)
		if providerAvailable(result.Providers) {
			available := true
			result.Status, result.Available = "available", &available
		}
		return result, nil
	}
	var failures []error
	sawNotFound := false
	for _, base := range servers {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, RDAPDomainURL(base, domain), nil)
		if err != nil {
			failures = append(failures, err)
			continue
		}
		resp, err := rdapHTTPClient.Do(req)
		if err != nil {
			failures = append(failures, err)
			continue
		}
		if resp.StatusCode == http.StatusNotFound {
			resp.Body.Close()
			sawNotFound = true
			continue
		}
		if resp.StatusCode == http.StatusOK {
			var record models.RDAPDomain
			decodeErr := json.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(&record)
			resp.Body.Close()
			if decodeErr != nil {
				failures = append(failures, fmt.Errorf("RDAP response: %w", decodeErr))
				continue
			}
			info := record.Info()
			result.Status, result.Available = "taken", boolPointer(false)
			result.Registrar, result.Registered, result.Expires = info.Registrar, info.Registered, info.Expires
			return result, nil
		}
		status := resp.StatusCode
		resp.Body.Close()
		failures = append(failures, fmt.Errorf("RDAP server returned HTTP %d", status))
	}
	if sawNotFound {
		available := true
		result.Status, result.Available = "available", &available
		result.Providers = CheckProviders(ctx, domain)
		return result, nil
	}
	result.Status = "unknown"
	result.Providers = CheckProviders(ctx, domain)
	if providerAvailable(result.Providers) {
		available := true
		result.Status, result.Available = "available", &available
		return result, nil
	}
	return result, errors.Join(failures...)
}

func boolPointer(value bool) *bool { return &value }

func providerAvailable(providers []models.ProviderResult) bool {
	for _, provider := range providers {
		if provider.Status == "available" {
			return true
		}
	}
	return false
}
