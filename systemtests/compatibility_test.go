package systemtests

import (
	"encoding/json"
	"testing"
	"time"

	billingapi "github.com/endless-net/billing/billingapi/v1"
	clientapi "github.com/endless-net/client-api/clientapi/v1"
	coordinatorapi "github.com/endless-net/coordinator/coordinatorapi/v1"
	identityapi "github.com/endless-net/identity/identityapi/v1"
	managementapi "github.com/endless-net/management/managementapi/v1"
	"github.com/endless-net/service-kit/event"
	signingapi "github.com/endless-net/signing/signingapi/v1"
)

func TestPinnedModulesShareCompatibilityBaseline(t *testing.T) {
	if clientapi.MapStreamProtocolVersion != 3 {
		t.Fatalf("map protocol = %d, want 3", clientapi.MapStreamProtocolVersion)
	}
	now := time.Unix(1, 0).UTC()
	envelope := event.Envelope{
		Version: event.EnvelopeVersion,
		ID:      "evt_1", Type: "billing.reservation.committed.v1", Source: "billing",
		OccurredAt: now, Payload: json.RawMessage(`{"reservation_id":"res_1"}`),
	}
	raw, err := json.Marshal(envelope)
	if err != nil {
		t.Fatal(err)
	}
	var managementEnvelope managementapi.EventEnvelope
	if err := json.Unmarshal(raw, &managementEnvelope); err != nil {
		t.Fatal(err)
	}
	if err := managementEnvelope.Validate(); err != nil {
		t.Fatalf("servicekit envelope is not accepted by Management: %v", err)
	}

	_ = identityapi.SessionIntrospectionRequest{SessionToken: "opaque"}
	_ = billingapi.ReserveRequest{
		IdempotencyKey: "command-1", AccountID: "account-1", Resource: "nodes", Amount: 1,
	}
	_ = coordinatorapi.ApplyProjectionCommand{
		CommandID: "command-1", NetworkID: "network-1", NodeID: "node-1",
		OccurredAt: now, Projection: json.RawMessage(`{}`),
	}
	_ = signingapi.SignRequest{
		RequestID: "command-1", Purpose: signingapi.PurposeNetworkMap,
		Payload: json.RawMessage(`{}`), IssuedAt: now, ExpiresAt: now.Add(time.Minute),
	}
}
