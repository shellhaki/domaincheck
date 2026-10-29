package internals

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"shellhaki/domaincheck/config"
	"shellhaki/domaincheck/models"
)

type cloudflareResponse struct {
	Success bool `json:"success"`
	Errors  []struct {
		Message string `json:"message"`
	} `json:"errors"`
	Result struct {
		Domains []struct {
			Name        string `json:"name"`
			Registrable bool   `json:"registrable"`
			Tier        string `json:"tier"`
			Reason      string `json:"reason"`
			Pricing     *struct {
				Currency     string `json:"currency"`
				Registration string `json:"registration_cost"`
				Renewal      string `json:"renewal_cost"`
			} `json:"pricing"`
		} `json:"domains"`
	} `json:"result"`
}

func CheckProviders(ctx context.Context, domain string) []models.ProviderResult {
	return []models.ProviderResult{checkCloudflare(ctx, domain)}
}

func checkCloudflare(ctx context.Context, domain string) models.ProviderResult {
	out := models.ProviderResult{Provider: "cloudflare"}
	if config.Vars.CloudflareAPIToken == "" || config.Vars.CloudflareAccountID == "" {
		out.Status, out.Error = "error", "provider is not configured"
		return out
	}
	body, _ := json.Marshal(map[string][]string{"domains": {domain}})
	endpoint := "https://api.cloudflare.com/client/v4/accounts/" + url.PathEscape(config.Vars.CloudflareAccountID) + "/registrar/domain-check"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		out.Status, out.Error = "error", err.Error()
		return out
	}
	req.Header.Set("Authorization", "Bearer "+config.Vars.CloudflareAPIToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := providerHTTPClient.Do(req)
	if err != nil {
		out.Status, out.Error = "error", safeError(err)
		return out
	}
	defer resp.Body.Close()
	var data cloudflareResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&data); err != nil {
		out.Status, out.Error = "error", "invalid provider response"
		return out
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 || !data.Success {
		out.Status, out.Error = "error", "provider rejected the request"
		if len(data.Errors) > 0 {
			out.Error = data.Errors[0].Message
		}
		return out
	}
	for _, d := range data.Result.Domains {
		if !strings.EqualFold(d.Name, domain) {
			continue
		}
		out.Premium = d.Tier == "premium" || d.Reason == "domain_premium"
		if d.Registrable {
			out.Status = "available"
			if d.Pricing != nil {
				out.Currency, out.RegistrationPrice, out.RenewalPrice = d.Pricing.Currency, d.Pricing.Registration, d.Pricing.Renewal
			}
		} else {
			out.Status = "unavailable"
			out.Error = d.Reason
		}
		return out
	}
	out.Status, out.Error = "error", "provider returned no result for domain"
	return out
}

var providerHTTPClient = &http.Client{Timeout: 10 * time.Second}

func safeError(err error) string {
	if err == nil {
		return "provider request failed"
	}
	return err.Error()
}
