package internals

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

const bootstrapFile = "dns.json"

// IANA encodes each service as a two-item array, so use a custom decoder.
type bootstrapRegistry struct {
	Services []json.RawMessage `json:"services"`
}

type Registry struct {
	mu      sync.RWMutex
	servers map[string][]string
	client  *http.Client
	url     string
}

func NewRegistry(client *http.Client, bootstrapURL string) *Registry {
	return &Registry{client: client, url: bootstrapURL, servers: make(map[string][]string)}
}

func (r *Registry) Load(ctx context.Context) error {
	if err := r.refresh(ctx); err == nil {
		return nil
	}
	data, err := os.ReadFile(bootstrapFile)
	if err != nil {
		return errors.New("could not load live IANA RDAP bootstrap or local dns.json fallback")
	}
	return r.replace(data)
}

func (r *Registry) RefreshLoop(ctx context.Context) {
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_ = r.refresh(ctx)
		}
	}
}

func (r *Registry) refresh(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, r.url, nil)
	if err != nil {
		return err
	}
	resp, err := r.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return errors.New("IANA bootstrap returned non-200 status")
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return err
	}
	if err := r.replace(data); err != nil {
		return err
	}
	// The live in-memory copy is authoritative for this process. Keep the
	// bundled file refreshed when the runtime directory is writable.
	_ = os.WriteFile(bootstrapFile, data, 0644)
	return nil
}

func (r *Registry) replace(data []byte) error {
	var doc bootstrapRegistry
	if err := json.Unmarshal(data, &doc); err != nil {
		return err
	}
	servers := make(map[string][]string)
	for _, raw := range doc.Services {
		var pair []json.RawMessage
		if err := json.Unmarshal(raw, &pair); err != nil || len(pair) != 2 {
			continue
		}
		var tlds, urls []string
		if json.Unmarshal(pair[0], &tlds) != nil || json.Unmarshal(pair[1], &urls) != nil || len(urls) == 0 {
			continue
		}
		for _, tld := range tlds {
			key := strings.ToLower(strings.TrimPrefix(tld, "."))
			if key != "" {
				servers[key] = append([]string(nil), urls...)
			}
		}
	}
	if len(servers) == 0 {
		return errors.New("IANA bootstrap contained no RDAP services")
	}
	r.mu.Lock()
	r.servers = servers
	r.mu.Unlock()
	return nil
}

func (r *Registry) Servers(tld string) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return append([]string(nil), r.servers[strings.ToLower(tld)]...)
}
