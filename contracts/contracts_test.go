package contracts

import (
	"encoding/json"
	"os"
	"testing"
)

func TestFrontendRuntimeConfigurationSchema(t *testing.T) {
	raw, err := os.ReadFile("frontend-runtime-config.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	var schema struct {
		Schema               string                     `json:"$schema"`
		AdditionalProperties bool                       `json:"additionalProperties"`
		Required             []string                   `json:"required"`
		Properties           map[string]json.RawMessage `json:"properties"`
	}
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatalf("decode frontend runtime schema: %v", err)
	}
	if schema.Schema != "https://json-schema.org/draft/2020-12/schema" {
		t.Fatalf("runtime schema dialect = %q", schema.Schema)
	}
	if schema.AdditionalProperties {
		t.Fatal("runtime configuration must reject unknown top-level properties")
	}
	for _, name := range []string{"schema_version", "management_api_base_path", "control_plane_url", "site_url", "admin_url", "admin_root"} {
		if _, ok := schema.Properties[name]; !ok {
			t.Errorf("runtime configuration schema missing %s", name)
		}
		if !contains(schema.Required, name) {
			t.Errorf("runtime configuration schema does not require %s", name)
		}
	}
	if _, legacy := schema.Properties["api_base_url"]; legacy {
		t.Fatal("runtime configuration schema still exposes api_base_url")
	}
}

func contains(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}
