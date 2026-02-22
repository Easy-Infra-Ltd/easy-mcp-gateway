package config

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
)

// validName matches alphanumeric, hyphens, and single underscores.
// Double underscores are reserved as the namespace separator.
var validName = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]*$`)

// Config is the top-level gateway configuration loaded from JSON.
type Config struct {
	Upstream     UpstreamConfig     `json:"upstream"`
	Downstream   []DownstreamConfig `json:"downstream"`
	Sanitization SanitizationConfig `json:"sanitization"`
}

// UpstreamConfig controls how LLM clients connect to the gateway.
type UpstreamConfig struct {
	Transport string     `json:"transport"` // "stdio" or "http"
	HTTP      HTTPConfig `json:"http"`
}

// HTTPConfig holds HTTP listener settings.
type HTTPConfig struct {
	Addr string `json:"addr"` // e.g. ":8080"
	Path string `json:"path"` // e.g. "/mcp"
}

// DownstreamConfig defines a single downstream MCP server.
type DownstreamConfig struct {
	Name         string              `json:"name"`
	Transport    string              `json:"transport"` // "stdio" or "http"
	Command      []string            `json:"command,omitempty"`
	URL          string              `json:"url,omitempty"`
	Sanitization *SanitizationConfig `json:"sanitization,omitempty"`
}

// SanitizationConfig controls the sanitization pipeline behaviour.
// When used at the root level it provides global defaults.
// When used per-downstream server, non-nil fields override the global.
type SanitizationConfig struct {
	MaxResponseChars               *int     `json:"maxResponseChars,omitempty"`
	EnablePromptInjectionDetection *bool    `json:"enablePromptInjectionDetection,omitempty"`
	EnableInvisibleTextRemoval     *bool    `json:"enableInvisibleTextRemoval,omitempty"`
	EnableURLValidation            *bool    `json:"enableURLValidation,omitempty"`
	EnableBoundaryInjection        *bool    `json:"enableBoundaryInjection,omitempty"`
	EnableSystemOverrideDetection  *bool    `json:"enableSystemOverrideDetection,omitempty"`
	DisableBuiltInPatterns         *bool    `json:"disableBuiltInPatterns,omitempty"`
	CustomInjectionPatterns        []string `json:"customInjectionPatterns,omitempty"`
}

const (
	TransportStdio = "stdio"
	TransportHTTP  = "http"

	DefaultMaxResponseChars = 16000
	DefaultHTTPAddr         = ":8080"
	DefaultHTTPPath         = "/mcp"
)

// Load reads and parses a JSON config file, applies defaults, and validates.
func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("reading config %s: %w", path, err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parsing config: %w", err)
	}

	applyDefaults(&cfg)

	if err := validate(cfg); err != nil {
		return Config{}, fmt.Errorf("validating config: %w", err)
	}

	return cfg, nil
}

func applyDefaults(cfg *Config) {
	if cfg.Upstream.Transport == "" {
		cfg.Upstream.Transport = TransportStdio
	}
	if cfg.Upstream.HTTP.Addr == "" {
		cfg.Upstream.HTTP.Addr = DefaultHTTPAddr
	}
	if cfg.Upstream.HTTP.Path == "" {
		cfg.Upstream.HTTP.Path = DefaultHTTPPath
	}

	if cfg.Sanitization.MaxResponseChars == nil {
		cfg.Sanitization.MaxResponseChars = intPtr(DefaultMaxResponseChars)
	}
	if cfg.Sanitization.EnablePromptInjectionDetection == nil {
		cfg.Sanitization.EnablePromptInjectionDetection = boolPtr(true)
	}
	if cfg.Sanitization.EnableInvisibleTextRemoval == nil {
		cfg.Sanitization.EnableInvisibleTextRemoval = boolPtr(true)
	}
	if cfg.Sanitization.EnableURLValidation == nil {
		cfg.Sanitization.EnableURLValidation = boolPtr(true)
	}
	if cfg.Sanitization.EnableBoundaryInjection == nil {
		cfg.Sanitization.EnableBoundaryInjection = boolPtr(true)
	}
	if cfg.Sanitization.EnableSystemOverrideDetection == nil {
		cfg.Sanitization.EnableSystemOverrideDetection = boolPtr(true)
	}
	if cfg.Sanitization.DisableBuiltInPatterns == nil {
		cfg.Sanitization.DisableBuiltInPatterns = boolPtr(false)
	}
}

func validate(cfg Config) error {
	if cfg.Upstream.Transport != TransportStdio && cfg.Upstream.Transport != TransportHTTP {
		return fmt.Errorf("upstream transport must be %q or %q, got %q",
			TransportStdio, TransportHTTP, cfg.Upstream.Transport)
	}

	if len(cfg.Downstream) == 0 {
		return fmt.Errorf("at least one downstream server is required")
	}

	names := make(map[string]struct{}, len(cfg.Downstream))
	for i, ds := range cfg.Downstream {
		if ds.Name == "" {
			return fmt.Errorf("downstream[%d]: name is required", i)
		}
		if !validName.MatchString(ds.Name) {
			return fmt.Errorf("downstream[%d]: name %q must match %s", i, ds.Name, validName.String())
		}
		if strings.Contains(ds.Name, "__") {
			return fmt.Errorf("downstream[%d]: name %q must not contain \"__\" (reserved separator)", i, ds.Name)
		}
		if _, exists := names[ds.Name]; exists {
			return fmt.Errorf("downstream[%d]: duplicate name %q", i, ds.Name)
		}
		names[ds.Name] = struct{}{}

		if ds.Transport != TransportStdio && ds.Transport != TransportHTTP {
			return fmt.Errorf("downstream[%d] (%s): transport must be %q or %q, got %q",
				i, ds.Name, TransportStdio, TransportHTTP, ds.Transport)
		}

		if ds.Transport == TransportStdio && len(ds.Command) == 0 {
			return fmt.Errorf("downstream[%d] (%s): command is required for stdio transport", i, ds.Name)
		}

		if ds.Transport == TransportHTTP && ds.URL == "" {
			return fmt.Errorf("downstream[%d] (%s): url is required for http transport", i, ds.Name)
		}
	}

	// Validate custom injection patterns are valid regexes.
	for i, pattern := range cfg.Sanitization.CustomInjectionPatterns {
		if _, err := regexp.Compile(pattern); err != nil {
			return fmt.Errorf("sanitization.customInjectionPatterns[%d]: invalid regex %q: %w", i, pattern, err)
		}
	}

	for di, ds := range cfg.Downstream {
		if ds.Sanitization == nil {
			continue
		}
		for i, pattern := range ds.Sanitization.CustomInjectionPatterns {
			if _, err := regexp.Compile(pattern); err != nil {
				return fmt.Errorf("downstream[%d] (%s) sanitization.customInjectionPatterns[%d]: invalid regex %q: %w",
					di, ds.Name, i, pattern, err)
			}
		}
	}

	return nil
}

// Merge returns a SanitizationConfig with per-server overrides applied on
// top of global defaults. Fields that are nil in the override use the global value.
func Merge(global, override *SanitizationConfig) SanitizationConfig {
	if override == nil {
		return *global
	}

	merged := *global

	if override.MaxResponseChars != nil {
		merged.MaxResponseChars = override.MaxResponseChars
	}
	if override.EnablePromptInjectionDetection != nil {
		merged.EnablePromptInjectionDetection = override.EnablePromptInjectionDetection
	}
	if override.EnableInvisibleTextRemoval != nil {
		merged.EnableInvisibleTextRemoval = override.EnableInvisibleTextRemoval
	}
	if override.EnableURLValidation != nil {
		merged.EnableURLValidation = override.EnableURLValidation
	}
	if override.EnableBoundaryInjection != nil {
		merged.EnableBoundaryInjection = override.EnableBoundaryInjection
	}
	if override.EnableSystemOverrideDetection != nil {
		merged.EnableSystemOverrideDetection = override.EnableSystemOverrideDetection
	}
	if override.DisableBuiltInPatterns != nil {
		merged.DisableBuiltInPatterns = override.DisableBuiltInPatterns
	}
	if len(override.CustomInjectionPatterns) > 0 {
		merged.CustomInjectionPatterns = override.CustomInjectionPatterns
	}

	return merged
}

func boolPtr(b bool) *bool { return &b }
func intPtr(i int) *int    { return &i }
