package config

import "os"

type Config struct {
	CloudflareAPIToken  string
	CloudflareAccountID string
	RDAPBootstrapURL    string
	Port                string
}

var Vars Config

func Load() {
	Vars = Config{
		CloudflareAPIToken:  os.Getenv("CLOUDFLARE_API_TOKEN"),
		CloudflareAccountID: os.Getenv("CLOUDFLARE_ACCOUNT_ID"),
		RDAPBootstrapURL:    envOr("RDAP_BOOTSTRAP_URL", "https://data.iana.org/rdap/dns.json"),
		Port:                envOr("PORT", "8000"),
	}
}

func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
