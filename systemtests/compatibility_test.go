package systemtests

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/endless-net/service-kit/event"
	billingapi "github.com/unng-lab/endlessnet-billing/billingapi/v1"
	coordinatorapi "github.com/unng-lab/endlessnet-coordinator/coordinatorapi/v1"
	identityapi "github.com/unng-lab/endlessnet-identity/identityapi/v1"
	managementapi "github.com/unng-lab/endlessnet-management/managementapi/v1"
	signingapi "github.com/unng-lab/endlessnet-signing/signingapi/v1"
	clientapi "github.com/unng-lab/endlessnet/clientapi/v2"
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
