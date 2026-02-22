package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_ValidConfig(t *testing.T) {
	cfg := `{
		"upstream": {"transport": "stdio"},
		"downstream": [
			{"name": "fs", "transport": "stdio", "command": ["echo", "hello"]}
		]
	}`

	path := writeTemp(t, cfg)
	got, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got.Upstream.Transport != TransportStdio {
		t.Errorf("upstream transport = %q, want %q", got.Upstream.Transport, TransportStdio)
	}
	if len(got.Downstream) != 1 {
		t.Fatalf("downstream count = %d, want 1", len(got.Downstream))
	}
	if got.Downstream[0].Name != "fs" {
		t.Errorf("downstream[0].name = %q, want %q", got.Downstream[0].Name, "fs")
	}
}

func TestLoad_DefaultsApplied(t *testing.T) {
	cfg := `{
		"downstream": [
			{"name": "a", "transport": "stdio", "command": ["x"]}
		]
	}`

	path := writeTemp(t, cfg)
	got, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got.Upstream.Transport != TransportStdio {
		t.Errorf("default upstream transport = %q, want %q", got.Upstream.Transport, TransportStdio)
	}
	if got.Upstream.HTTP.Addr != DefaultHTTPAddr {
		t.Errorf("default http addr = %q, want %q", got.Upstream.HTTP.Addr, DefaultHTTPAddr)
	}
	if got.Upstream.HTTP.Path != DefaultHTTPPath {
		t.Errorf("default http path = %q, want %q", got.Upstream.HTTP.Path, DefaultHTTPPath)
	}
	if *got.Sanitization.MaxResponseChars != DefaultMaxResponseChars {
		t.Errorf("default maxResponseChars = %d, want %d", *got.Sanitization.MaxResponseChars, DefaultMaxResponseChars)
	}
	if !*got.Sanitization.EnablePromptInjectionDetection {
		t.Error("default enablePromptInjectionDetection should be true")
	}
	if !*got.Sanitization.EnableInvisibleTextRemoval {
		t.Error("default enableInvisibleTextRemoval should be true")
	}
	if !*got.Sanitization.EnableURLValidation {
		t.Error("default enableURLValidation should be true")
	}
	if !*got.Sanitization.EnableBoundaryInjection {
		t.Error("default enableBoundaryInjection should be true")
	}
	if !*got.Sanitization.EnableSystemOverrideDetection {
		t.Error("default enableSystemOverrideDetection should be true")
	}
	if *got.Sanitization.DisableBuiltInPatterns {
		t.Error("default disableBuiltInPatterns should be false")
	}
}

func TestLoad_HTTPUpstream(t *testing.T) {
	cfg := `{
		"upstream": {"transport": "http", "http": {"addr": ":9090", "path": "/api"}},
		"downstream": [
			{"name": "a", "transport": "http", "url": "https://example.com/mcp"}
		]
	}`

	path := writeTemp(t, cfg)
	got, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got.Upstream.Transport != TransportHTTP {
		t.Errorf("transport = %q, want %q", got.Upstream.Transport, TransportHTTP)
	}
	if got.Upstream.HTTP.Addr != ":9090" {
		t.Errorf("addr = %q, want %q", got.Upstream.HTTP.Addr, ":9090")
	}
}

func TestLoad_NoDownstream(t *testing.T) {
	cfg := `{"downstream": []}`
	path := writeTemp(t, cfg)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for empty downstream")
	}
}

func TestLoad_DuplicateNames(t *testing.T) {
	cfg := `{
		"downstream": [
			{"name": "a", "transport": "stdio", "command": ["x"]},
			{"name": "a", "transport": "stdio", "command": ["y"]}
		]
	}`
	path := writeTemp(t, cfg)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for duplicate names")
	}
}

func TestLoad_StdioMissingCommand(t *testing.T) {
	cfg := `{
		"downstream": [
			{"name": "a", "transport": "stdio"}
		]
	}`
	path := writeTemp(t, cfg)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for stdio without command")
	}
}

func TestLoad_HTTPMissingURL(t *testing.T) {
	cfg := `{
		"downstream": [
			{"name": "a", "transport": "http"}
		]
	}`
	path := writeTemp(t, cfg)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for http without url")
	}
}

func TestLoad_InvalidTransport(t *testing.T) {
	cfg := `{
		"upstream": {"transport": "grpc"},
		"downstream": [
			{"name": "a", "transport": "stdio", "command": ["x"]}
		]
	}`
	path := writeTemp(t, cfg)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for invalid upstream transport")
	}
}

func TestLoad_InvalidRegex(t *testing.T) {
	cfg := `{
		"downstream": [
			{"name": "a", "transport": "stdio", "command": ["x"]}
		],
		"sanitization": {
			"customInjectionPatterns": ["[invalid"]
		}
	}`
	path := writeTemp(t, cfg)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for invalid regex")
	}
}

func TestLoad_FileNotFound(t *testing.T) {
	_, err := Load("/nonexistent/config.json")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestLoad_InvalidJSON(t *testing.T) {
	path := writeTemp(t, `{not json}`)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestLoad_NameContainsDoubleUnderscore(t *testing.T) {
	cfg := `{
		"downstream": [
			{"name": "a__b", "transport": "stdio", "command": ["x"]}
		]
	}`
	path := writeTemp(t, cfg)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for name containing __")
	}
}

func TestLoad_NameInvalidChars(t *testing.T) {
	cfg := `{
		"downstream": [
			{"name": "has spaces", "transport": "stdio", "command": ["x"]}
		]
	}`
	path := writeTemp(t, cfg)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for name with invalid chars")
	}
}

func TestLoad_NameWithHyphensAndUnderscores(t *testing.T) {
	cfg := `{
		"downstream": [
			{"name": "my-server_1", "transport": "stdio", "command": ["x"]}
		]
	}`
	path := writeTemp(t, cfg)
	_, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error for valid name: %v", err)
	}
}

func TestMerge_NilOverride(t *testing.T) {
	global := SanitizationConfig{
		MaxResponseChars: intPtr(16000),
	}
	merged := Merge(&global, nil)
	if *merged.MaxResponseChars != 16000 {
		t.Errorf("maxResponseChars = %d, want 16000", *merged.MaxResponseChars)
	}
}

func TestMerge_OverrideFields(t *testing.T) {
	global := SanitizationConfig{
		MaxResponseChars:               intPtr(16000),
		EnablePromptInjectionDetection: boolPtr(true),
		EnableBoundaryInjection:        boolPtr(true),
	}
	override := SanitizationConfig{
		MaxResponseChars:        intPtr(8000),
		EnableBoundaryInjection: boolPtr(false),
	}

	merged := Merge(&global, &override)

	if *merged.MaxResponseChars != 8000 {
		t.Errorf("maxResponseChars = %d, want 8000", *merged.MaxResponseChars)
	}
	if !*merged.EnablePromptInjectionDetection {
		t.Error("enablePromptInjectionDetection should remain true from global")
	}
	if *merged.EnableBoundaryInjection {
		t.Error("enableBoundaryInjection should be false from override")
	}
}

func TestMerge_CustomPatternsOverride(t *testing.T) {
	global := SanitizationConfig{
		CustomInjectionPatterns: []string{"global_pattern"},
	}
	override := SanitizationConfig{
		CustomInjectionPatterns: []string{"override_pattern"},
	}

	merged := Merge(&global, &override)

	if len(merged.CustomInjectionPatterns) != 1 || merged.CustomInjectionPatterns[0] != "override_pattern" {
		t.Errorf("custom patterns = %v, want [override_pattern]", merged.CustomInjectionPatterns)
	}
}

func writeTemp(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("writing temp config: %v", err)
	}
	return path
}
