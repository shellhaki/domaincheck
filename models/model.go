package models

type RDAPDomain struct {
	Events []struct {
		Action string `json:"eventAction"`
		Date   string `json:"eventDate"`
	} `json:"events"`
	Entities []struct {
		Roles      []string `json:"roles"`
		VCardArray []any    `json:"vcardArray"`
	} `json:"entities"`
}

type DomainInfo struct {
	Available  bool   `json:"available"`
	Registrar  string `json:"registrar,omitempty"`
	Registered string `json:"registered,omitempty"`
	Expires    string `json:"expires,omitempty"`
}

func (d RDAPDomain) Info() DomainInfo {
	info := DomainInfo{Available: false}
	for _, e := range d.Events {
		switch e.Action {
		case "registration":
			info.Registered = e.Date
		case "expiration":
			info.Expires = e.Date
		}
	}
	for _, e := range d.Entities {
		for _, role := range e.Roles {
			if role != "registrar" || len(e.VCardArray) < 2 {
				continue
			}
			props, _ := e.VCardArray[1].([]any)
			for _, prop := range props {
				field, ok := prop.([]any)
				if ok && len(field) >= 4 && field[0] == "fn" {
					info.Registrar, _ = field[3].(string)
					if info.Registrar != "" {
						return info
					}
				}
			}
		}
	}
	return info
}

type DomainResult struct {
	Domain     string           `json:"domain"`
	Status     string           `json:"status"` // available, taken, or unknown
	Available  *bool            `json:"available,omitempty"`
	Registrar  string           `json:"registrar,omitempty"`
	Registered string           `json:"registered,omitempty"`
	Expires    string           `json:"expires,omitempty"`
	Providers  []ProviderResult `json:"providers,omitempty"`
	Cached     bool             `json:"cached,omitempty"`
}

type ProviderResult struct {
	Provider          string `json:"provider"`
	Status            string `json:"status"` // available, unavailable, or error
	Premium           bool   `json:"premium,omitempty"`
	Currency          string `json:"currency,omitempty"`
	RegistrationPrice string `json:"registration_price,omitempty"`
	RenewalPrice      string `json:"renewal_price,omitempty"`
	Error             string `json:"error,omitempty"`
}
