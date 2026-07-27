package contracts

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"endlessnet/internal/management"
)

type openAPIDocument struct {
	OpenAPI         string         `json:"openapi"`
	ContractStatus  string         `json:"x-endlessnet-contract-status"`
	ContractVersion string         `json:"x-endlessnet-contract-version"`
	Info            map[string]any `json:"info"`
	Servers         []struct {
		URL string `json:"url"`
	} `json:"servers"`
	Paths      map[string]map[string]any `json:"paths"`
	Security   []map[string]any          `json:"security"`
	Components struct {
		Schemas map[string]any `json:"schemas"`
	} `json:"components"`
}

func TestFrontendOpenAPIContract(t *testing.T) {
	raw, err := os.ReadFile("frontend-api.openapi.json")
	if err != nil {
		t.Fatal(err)
	}
	var spec openAPIDocument
	if err := json.Unmarshal(raw, &spec); err != nil {
		t.Fatalf("decode frontend OpenAPI: %v", err)
	}
	if spec.OpenAPI != "3.0.0" {
		t.Fatalf("openapi = %q, want 3.0.0", spec.OpenAPI)
	}
	if len(spec.Paths) == 0 {
		t.Fatal("frontend OpenAPI has no paths")
	}
	if len(spec.Servers) != 1 || spec.Servers[0].URL != "https://admin.endlessnet.ru/api/v1" {
		t.Fatalf("frontend OpenAPI servers = %#v", spec.Servers)
	}
	if spec.ContractStatus != "stable" {
		t.Fatalf("frontend contract status = %q, want stable", spec.ContractStatus)
	}
	if spec.ContractVersion != "4.0.0" {
		t.Fatalf("frontend contract version = %q, want 4.0.0", spec.ContractVersion)
	}
	if len(spec.Components.Schemas) == 0 {
		t.Fatal("frontend OpenAPI has no component schemas")
	}
	if _, legacy := spec.Components.Schemas["JsonValue"]; legacy {
		t.Fatal("frontend OpenAPI still contains the untyped JsonValue placeholder")
	}

	for _, operation := range frontendOperations() {
		methods, ok := spec.Paths[operation.path]
		if !ok {
			t.Errorf("frontend OpenAPI missing path %s", operation.path)
			continue
		}
		rawOperation, ok := methods[operation.method]
		if !ok {
			t.Errorf("frontend OpenAPI missing %s %s", strings.ToUpper(operation.method), operation.path)
			continue
		}
		encoded, err := json.Marshal(rawOperation)
		if err != nil {
			t.Fatal(err)
		}
		var details struct {
			OperationID string                     `json:"operationId"`
			Responses   map[string]json.RawMessage `json:"responses"`
		}
		if err := json.Unmarshal(encoded, &details); err != nil {
			t.Fatal(err)
		}
		if details.OperationID != operation.id {
			t.Errorf("%s %s operationId = %q, want %q", strings.ToUpper(operation.method), operation.path, details.OperationID, operation.id)
		}
		if len(details.Responses) == 0 {
			t.Errorf("%s %s has no responses", strings.ToUpper(operation.method), operation.path)
		}
		validateContractReferences(t, rawOperation, spec.Components.Schemas)
	}

	for path := range spec.Paths {
		if !management.OwnsPublicPath(path) {
			t.Errorf("frontend contract path has no Management handler: %s", path)
		}
		for _, forbidden := range []string{"/internal", "/nodes", "/maps", "/signing", "/relays", "/server-key"} {
			if path == forbidden || strings.HasPrefix(path, forbidden+"/") {
				t.Errorf("frontend contract exposes non-frontend endpoint %s", path)
			}
		}
	}
}

func validateContractReferences(t *testing.T, value any, schemas map[string]any) {
	t.Helper()
	switch current := value.(type) {
	case map[string]any:
		for key, child := range current {
			if key == "$ref" {
				ref, ok := child.(string)
				if !ok {
					t.Errorf("OpenAPI reference has type %T", child)
					continue
				}
				const prefix = "#/components/schemas/"
				if strings.HasPrefix(ref, prefix) {
					name := strings.TrimPrefix(ref, prefix)
					if _, ok := schemas[name]; !ok {
						t.Errorf("OpenAPI reference points to missing schema %s", name)
					}
				}
			}
			validateContractReferences(t, child, schemas)
		}
	case []any:
		for _, child := range current {
			validateContractReferences(t, child, schemas)
		}
	}
}

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

func TestAdminMachineExposesClientVersionPolicyResult(t *testing.T) {
	raw, err := os.ReadFile("frontend-api.openapi.json")
	if err != nil {
		t.Fatal(err)
	}
	var document struct {
		Components struct {
			Schemas map[string]struct {
				Properties map[string]json.RawMessage `json:"properties"`
				Required   []string                   `json:"required"`
			} `json:"schemas"`
		} `json:"components"`
	}
	if err := json.Unmarshal(raw, &document); err != nil {
		t.Fatal(err)
	}
	machine, ok := document.Components.Schemas["AdminMachine"]
	if !ok {
		t.Fatal("frontend OpenAPI missing AdminMachine")
	}
	if !contains(machine.Required, "client_version_status") {
		t.Fatal("AdminMachine.client_version_status must be required")
	}
	if !contains(machine.Required, "recommended_client_version") {
		t.Fatal("AdminMachine.recommended_client_version must be required")
	}

	var status struct {
		Type string   `json:"type"`
		Enum []string `json:"enum"`
	}
	if err := json.Unmarshal(machine.Properties["client_version_status"], &status); err != nil {
		t.Fatal(err)
	}
	if status.Type != "string" ||
		!contains(status.Enum, "current") ||
		!contains(status.Enum, "outdated") ||
		!contains(status.Enum, "unknown") {
		t.Fatalf("client_version_status schema = %#v", status)
	}

	var recommended struct {
		Type     string `json:"type"`
		Nullable bool   `json:"nullable"`
	}
	if err := json.Unmarshal(machine.Properties["recommended_client_version"], &recommended); err != nil {
		t.Fatal(err)
	}
	if recommended.Type != "string" || !recommended.Nullable {
		t.Fatalf("recommended_client_version schema = %#v", recommended)
	}
}

type frontendOperation struct {
	method string
	path   string
	id     string
}

func frontendOperations() []frontendOperation {
	return []frontendOperation{
		{"get", "/status", "getManagementStatus"},
		{"get", "/auth/providers", "listAuthProviders"},
		{"get", "/auth/login", "startBrowserLogin"},
		{"get", "/auth/callback", "completeBrowserLogin"},
		{"get", "/auth/me", "getCurrentUser"},
		{"post", "/auth/logout", "logout"},
		{"get", "/accounts", "listAccounts"},
		{"patch", "/accounts/{account_id}", "patchAccount"},
		{"get", "/accounts/{account_id}/members", "listAccountMembers"},
		{"post", "/accounts/{account_id}/invites", "inviteAccountMember"},
		{"patch", "/accounts/{account_id}/members/{user_id}", "updateAccountMember"},
		{"delete", "/accounts/{account_id}/members/{user_id}", "removeAccountMember"},
		{"get", "/networks", "listNetworks"},
		{"post", "/networks", "createNetwork"},
		{"get", "/networks/{network_id}/nodes", "listNetworkNodes"},
		{"get", "/admin/accounts/{account_id}/machines", "listMachines"},
		{"get", "/admin/machines/{machine_id}", "getMachine"},
		{"patch", "/admin/machines/{machine_id}", "patchMachine"},
		{"delete", "/admin/machines/{machine_id}", "deleteMachine"},
		{"post", "/admin/accounts/{account_id}/join-tokens", "createJoinToken"},
		{"delete", "/admin/accounts/{account_id}/join-tokens/{token_id}", "revokeJoinToken"},
		{"get", "/admin/accounts/{account_id}/apps", "listApps"},
		{"post", "/admin/accounts/{account_id}/apps", "createApp"},
		{"patch", "/admin/apps/{app_id}", "updateApp"},
		{"delete", "/admin/apps/{app_id}", "deleteApp"},
		{"get", "/admin/accounts/{account_id}/services", "listServices"},
		{"post", "/admin/accounts/{account_id}/services", "createService"},
		{"patch", "/admin/services/{service_id}", "updateService"},
		{"delete", "/admin/services/{service_id}", "deleteService"},
		{"get", "/admin/accounts/{account_id}/policy", "getPolicy"},
		{"put", "/admin/accounts/{account_id}/policy", "savePolicy"},
		{"post", "/admin/accounts/{account_id}/policy/validate", "validatePolicy"},
		{"post", "/admin/accounts/{account_id}/policy/preview", "previewPolicy"},
		{"post", "/admin/accounts/{account_id}/policy/tests/run", "runPolicyTests"},
		{"get", "/admin/accounts/{account_id}/dns", "getDNSSettings"},
		{"patch", "/admin/accounts/{account_id}/dns", "patchDNSSettings"},
		{"post", "/admin/accounts/{account_id}/dns/nameservers", "createDNSNameserver"},
		{"patch", "/admin/accounts/{account_id}/dns/nameservers/{resource_id}", "updateDNSNameserver"},
		{"delete", "/admin/accounts/{account_id}/dns/nameservers/{resource_id}", "deleteDNSNameserver"},
		{"post", "/admin/accounts/{account_id}/dns/search-domains", "createDNSSearchDomain"},
		{"patch", "/admin/accounts/{account_id}/dns/search-domains/{resource_id}", "updateDNSSearchDomain"},
		{"delete", "/admin/accounts/{account_id}/dns/search-domains/{resource_id}", "deleteDNSSearchDomain"},
		{"get", "/admin/accounts/{account_id}/audit", "listAuditEvents"},
		{"get", "/admin/accounts/{account_id}/flow-logs", "listFlowLogs"},
		{"post", "/admin/accounts/{account_id}/flow-logs/settings", "updateFlowLogSettings"},
		{"get", "/admin/accounts/{account_id}/log-streams", "listLogStreams"},
		{"post", "/admin/accounts/{account_id}/log-streams", "createLogStream"},
		{"patch", "/admin/accounts/{account_id}/log-streams/{resource_id}", "updateLogStream"},
		{"delete", "/admin/accounts/{account_id}/log-streams/{resource_id}", "deleteLogStream"},
		{"get", "/billing/plans", "listBillingPlans"},
		{"get", "/accounts/{account_id}/billing/subscription", "getBillingSubscription"},
		{"get", "/accounts/{account_id}/billing/usage", "getBillingUsage"},
		{"get", "/accounts/{account_id}/billing/invoices", "listBillingInvoices"},
		{"get", "/accounts/{account_id}/billing/legal", "getBillingLegalProfile"},
		{"post", "/accounts/{account_id}/billing/change-plan", "changeBillingPlan"},
		{"get", "/accounts/{account_id}/license", "getLicense"},
		{"post", "/accounts/{account_id}/billing/checkout", "createBillingCheckout"},
		{"get", "/accounts/{account_id}/billing/checkout/{checkout_id}", "getBillingCheckout"},
		{"get", "/admin/accounts/{account_id}/trust-credentials", "listTrustCredentials"},
		{"post", "/admin/accounts/{account_id}/trust-credentials", "createTrustCredential"},
		{"patch", "/admin/accounts/{account_id}/trust-credentials/{resource_id}", "updateTrustCredential"},
		{"delete", "/admin/accounts/{account_id}/trust-credentials/{resource_id}", "deleteTrustCredential"},
		{"get", "/admin/accounts/{account_id}/webhooks", "listWebhooks"},
		{"post", "/admin/accounts/{account_id}/webhooks", "createWebhook"},
		{"patch", "/admin/accounts/{account_id}/webhooks/{resource_id}", "updateWebhook"},
		{"delete", "/admin/accounts/{account_id}/webhooks/{resource_id}", "deleteWebhook"},
		{"post", "/admin/accounts/{account_id}/webhooks/{resource_id}/test", "testWebhook"},
	}
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
