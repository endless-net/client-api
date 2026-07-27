package contracts

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestFrontendSchemasDoNotDependOnRuntimeGoTypes(t *testing.T) {
	raw, err := os.ReadFile("frontend-api.openapi.json")
	if err != nil {
		t.Fatal(err)
	}
	var document struct {
		Components struct {
			Schemas map[string]json.RawMessage `json:"schemas"`
		} `json:"components"`
	}
	if err := json.Unmarshal(raw, &document); err != nil {
		t.Fatal(err)
	}
	for name, schema := range document.Components.Schemas {
		if len(schema) == 0 || string(schema) == "{}" {
			t.Errorf("schema %s is empty", name)
		}
		if strings.Contains(string(schema), `"additionalProperties":true`) {
			t.Errorf("schema %s has an unbounded object contract", name)
		}
	}
}
