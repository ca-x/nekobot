package criteria

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

// Schema validates criteria sets.
type Schema struct{}

// NewSchema creates a criteria schema validator.
func NewSchema() *Schema { return &Schema{} }

// Validate validates one criteria set.
func (s *Schema) Validate(set Set) error {
	if len(set.Criteria) == 0 {
		return fmt.Errorf("criteria cannot be empty")
	}
	ids := make(map[string]struct{}, len(set.Criteria))
	for _, item := range set.Criteria {
		if strings.TrimSpace(item.ID) == "" {
			return fmt.Errorf("criterion id is required")
		}
		if _, exists := ids[item.ID]; exists {
			return fmt.Errorf("criterion id %q must be unique", item.ID)
		}
		ids[item.ID] = struct{}{}
		if strings.TrimSpace(item.Title) == "" {
			return fmt.Errorf("criterion %q title is required", item.ID)
		}
		if item.Scope.Kind == "" {
			return fmt.Errorf("criterion %q scope kind is required", item.ID)
		}
		if strings.TrimSpace(item.Scope.Source) == "" {
			return fmt.Errorf("criterion %q scope source is required", item.ID)
		}
		if item.Type == TypeManualConfirmation {
			prompt, _ := item.Definition["prompt"].(string)
			if strings.TrimSpace(prompt) == "" {
				return fmt.Errorf("criterion %q manual_confirmation prompt is required", item.ID)
			}
			continue
		}
		if item.Type == TypeCommand {
			command, _ := item.Definition["command"].(string)
			if strings.TrimSpace(command) == "" {
				return fmt.Errorf("criterion %q command is required", item.ID)
			}
			if _, ok := item.Definition["expect_exit_code"]; !ok {
				return fmt.Errorf("criterion %q expect_exit_code is required", item.ID)
			}
			continue
		}
		if item.Type == TypeFileExists {
			path, _ := item.Definition["path"].(string)
			if strings.TrimSpace(path) == "" {
				return fmt.Errorf("criterion %q path is required", item.ID)
			}
			continue
		}
		if item.Type == TypeFileContains {
			path, _ := item.Definition["path"].(string)
			contains, _ := item.Definition["contains"].(string)
			if strings.TrimSpace(path) == "" || strings.TrimSpace(contains) == "" {
				return fmt.Errorf("criterion %q path and contains are required", item.ID)
			}
			continue
		}
		if item.Type == TypeHTTPCheck {
			targetURL, _ := item.Definition["url"].(string)
			if strings.TrimSpace(targetURL) == "" {
				return fmt.Errorf("criterion %q url is required", item.ID)
			}
			if err := ValidateHTTPURL(strings.TrimSpace(targetURL)); err != nil {
				return fmt.Errorf("criterion %q url is not allowed: %w", item.ID, err)
			}
			returnStatus, hasStatus := item.Definition["expect_status"]
			returnBody, hasBody := item.Definition["body_contains"]
			if !hasStatus && !hasBody {
				return fmt.Errorf("criterion %q http_check requires expect_status or body_contains", item.ID)
			}
			if hasStatus {
				switch returnStatus.(type) {
				case int, int32, int64, float64:
				default:
					return fmt.Errorf("criterion %q expect_status must be numeric", item.ID)
				}
			}
			if hasBody {
				if bodyText, _ := returnBody.(string); strings.TrimSpace(bodyText) == "" {
					return fmt.Errorf("criterion %q body_contains must be non-empty", item.ID)
				}
			}
			continue
		}
		return fmt.Errorf("criterion %q has unsupported type %q", item.ID, item.Type)
	}
	return nil
}

// ValidateHTTPURL rejects non-http(s) and local/private targets for http_check.
func ValidateHTTPURL(raw string) error {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return fmt.Errorf("parse url: %w", err)
	}
	switch strings.ToLower(parsed.Scheme) {
	case "http", "https":
	default:
		return fmt.Errorf("scheme must be http or https")
	}
	host := strings.TrimSpace(parsed.Hostname())
	if host == "" {
		return fmt.Errorf("host is required")
	}
	if strings.EqualFold(host, "localhost") {
		return fmt.Errorf("localhost is not allowed")
	}
	ip := net.ParseIP(host)
	if ip == nil {
		if strings.Contains(strings.ToLower(host), "localhost") || strings.Contains(strings.ToLower(host), "metadata") {
			return fmt.Errorf("local or metadata hostnames are not allowed")
		}
		return nil
	}
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() {
		return fmt.Errorf("private or local IPs are not allowed")
	}
	if host == "169.254.169.254" {
		return fmt.Errorf("metadata IP is not allowed")
	}
	return nil
}

// IsBlockedIP reports whether an IP target is unsafe for server-side HTTP checks.
func IsBlockedIP(ip net.IP) bool {
	if ip == nil {
		return false
	}
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified()
}
