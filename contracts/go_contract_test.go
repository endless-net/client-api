package contracts

import (
	"encoding/json"
	"os"
	"reflect"
	"slices"
	"sort"
	"strings"
	"testing"

	"endlessnet/internal/control"
	"endlessnet/internal/management"
)

func TestFrontendOpenAPISchemasTrackProducerDTOs(t *testing.T) {
	raw, err := os.ReadFile("frontend-api.openapi.json")
	if err != nil {
		t.Fatal(err)
	}
	var document struct {
		Components struct {
			Schemas map[string]struct {
				Properties map[string]any `json:"properties"`
				Required   []string       `json:"required"`
			} `json:"schemas"`
		} `json:"components"`
	}
	if err := json.Unmarshal(raw, &document); err != nil {
		t.Fatal(err)
	}

	for name, value := range producerDTOs() {
		schema, ok := document.Components.Schemas[name]
		if !ok {
			t.Errorf("frontend OpenAPI missing producer DTO schema %s", name)
			continue
		}
		properties, required := jsonFields(reflect.TypeOf(value))
		actualProperties := make([]string, 0, len(schema.Properties))
		for property := range schema.Properties {
			actualProperties = append(actualProperties, property)
		}
		sort.Strings(actualProperties)
		sort.Strings(schema.Required)
		if !slices.Equal(actualProperties, properties) {
			t.Errorf("schema %s properties = %v, producer DTO fields = %v", name, actualProperties, properties)
		}
		if !slices.Equal(schema.Required, required) {
			t.Errorf("schema %s required = %v, producer DTO required fields = %v", name, schema.Required, required)
		}
	}
}

func producerDTOs() map[string]any {
	return map[string]any{
		"Account":                     control.Account{},
		"AccountLegalProfile":         control.AccountLegalProfile{},
		"AccountMembership":           control.AccountMembership{},
		"AdminMachine":                control.AdminMachine{},
		"AdvertisedService":           control.AdvertisedService{},
		"AppConnectorApp":             control.AppConnectorApp{},
		"AppListResponse":             control.AppListResponse{},
		"AppRequest":                  control.AppRequest{},
		"AuditEvent":                  control.AuditEvent{},
		"AuthProvider":                management.BrowserAuthProvider{},
		"AuthProvidersResponse":       management.BrowserAuthProvidersResponse{},
		"BillingCheckoutRequest":      control.BillingCheckoutRequest{},
		"ChangePlanRequest":           control.ChangePlanRequest{},
		"CheckoutSession":             control.CheckoutSession{},
		"CreateJoinTokenRequest":      control.CreateJoinTokenRequest{},
		"CreateJoinTokenResponse":     control.CreateJoinTokenResponse{},
		"CreateNetworkRequest":        control.CreateNetworkRequest{},
		"DNSNameserver":               control.DNSNameserver{},
		"DNSNameserverRequest":        control.DNSNameserverRequest{},
		"DNSSearchDomain":             control.DNSSearchDomain{},
		"DNSSearchDomainRequest":      control.DNSSearchDomainRequest{},
		"DNSSettings":                 control.DNSSettings{},
		"DNSSettingsRequest":          control.DNSSettingsRequest{},
		"FlowLogEvent":                control.FlowLogEvent{},
		"FlowLogListResponse":         control.FlowLogListResponse{},
		"FlowLogSettings":             control.FlowLogSettings{},
		"FlowLogSettingsRequest":      control.FlowLogSettingsRequest{},
		"InviteAccountMemberRequest":  control.InviteAccountMemberRequest{},
		"Invoice":                     control.Invoice{},
		"InvoiceItem":                 control.InvoiceItem{},
		"LicenseKey":                  control.LicenseKey{},
		"LogStream":                   control.LogStream{},
		"LogStreamRequest":            control.LogStreamRequest{},
		"MachineListResponse":         control.MachineListResponse{},
		"Network":                     control.Network{},
		"Node":                        control.Node{},
		"PageInfo":                    control.PageInfo{},
		"PatchAccountRequest":         control.PatchAccountRequest{},
		"PatchMachineRequest":         control.PatchMachineRequest{},
		"Plan":                        control.Plan{},
		"PolicyFile":                  control.PolicyFile{},
		"PolicyPreviewResponse":       control.PolicyPreviewResponse{},
		"PolicySaveRequest":           control.PolicySaveRequest{},
		"PolicyTestResult":            control.PolicyTestResult{},
		"PolicyTestRunResponse":       control.PolicyTestRunResponse{},
		"PolicyValidationResponse":    control.PolicyValidationResponse{},
		"ServiceListResponse":         control.ServiceListResponse{},
		"ServicePort":                 control.ServicePort{},
		"ServiceRequest":              control.ServiceRequest{},
		"Subscription":                control.Subscription{},
		"TrustCredential":             control.TrustCredential{},
		"TrustCredentialListResponse": control.TrustCredentialListResponse{},
		"TrustCredentialRequest":      control.TrustCredentialRequest{},
		"UpdateAccountMemberRequest":  control.UpdateAccountMemberRequest{},
		"UsageSnapshot":               control.UsageSnapshot{},
		"WebhookDelivery":             control.WebhookDelivery{},
		"WebhookEndpoint":             control.WebhookEndpoint{},
		"WebhookEndpointListResponse": control.WebhookEndpointListResponse{},
		"WebhookEndpointRequest":      control.WebhookEndpointRequest{},
	}
}

func jsonFields(valueType reflect.Type) ([]string, []string) {
	if valueType.Kind() == reflect.Pointer {
		valueType = valueType.Elem()
	}
	properties := []string{}
	required := []string{}
	for i := 0; i < valueType.NumField(); i++ {
		field := valueType.Field(i)
		if !field.IsExported() {
			continue
		}
		parts := strings.Split(field.Tag.Get("json"), ",")
		name := parts[0]
		if name == "-" {
			continue
		}
		if name == "" {
			name = field.Name
		}
		properties = append(properties, name)
		if !contains(parts[1:], "omitempty") {
			required = append(required, name)
		}
	}
	sort.Strings(properties)
	sort.Strings(required)
	return properties, required
}
