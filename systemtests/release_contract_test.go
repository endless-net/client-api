package systemtests

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/endless-net/client-api/systemtests/releasecontract"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

const releaseFixtureDirectory = "../release/fixtures/v1"

type releaseContractFixture struct {
	candidateRaw  []byte
	provenanceRaw []byte
	systemTestRaw []byte
	envelopeRaw   []byte
	candidate     releasecontract.Candidate
	provenance    releasecontract.CandidateProvenance
	systemTest    releasecontract.SystemTestEvidence
	request       releasecontract.PromotionRequest
	envelope      releasecontract.ReleasedEnvelope
}

type releaseMigrationInventory struct {
	State     string `json:"state"`
	Authority struct {
		Repository string `json:"repository"`
		Revision   string `json:"revision"`
	} `json:"authority"`
	Source struct {
		Repository string `json:"repository"`
		Revision   string `json:"revision"`
		Role       string `json:"role"`
	} `json:"source"`
	Target struct {
		Repository       string `json:"repository"`
		BaselineRevision string `json:"baseline_revision"`
		Role             string `json:"role"`
	} `json:"target"`
	Records []struct {
		SourcePath string `json:"source_path"`
		TargetPath string `json:"target_path"`
		SHA256     string `json:"sha256"`
	} `json:"records"`
	CutoverGates []struct {
		Status   string `json:"status"`
		Evidence any    `json:"evidence"`
	} `json:"cutover_gates"`
	Completion struct {
		Status               string `json:"status"`
		ProductionAuthorized bool   `json:"production_authorized"`
	} `json:"completion"`
}

type releaseMigrationInventoryV2 struct {
	State  string `json:"state"`
	Source struct {
		Repository            string `json:"repository"`
		Revision              string `json:"revision"`
		OwnershipStopRevision string `json:"ownership_stop_revision"`
		Role                  string `json:"role"`
	} `json:"source"`
	Target struct {
		Repository             string `json:"repository"`
		BaselineRevision       string `json:"baseline_revision"`
		ImplementationRevision string `json:"implementation_revision"`
		Role                   string `json:"role"`
	} `json:"target"`
	PreviousInventory struct {
		Path        string `json:"path"`
		SHA256      string `json:"sha256"`
		Disposition string `json:"disposition"`
	} `json:"previous_inventory"`
	DestinationContract struct {
		Revision string `json:"revision"`
		Records  []struct {
			Class  string `json:"class"`
			Path   string `json:"path"`
			SHA256 string `json:"sha256"`
		} `json:"records"`
		Capabilities []struct {
			ID            string   `json:"id"`
			Status        string   `json:"status"`
			EvidencePaths []string `json:"evidence_paths"`
		} `json:"capabilities"`
		ConsumerResolution struct {
			Schemas struct {
				Revision          string `json:"revision"`
				CopyIntoConsumers bool   `json:"copy_into_consumers"`
			} `json:"schemas"`
			Fixtures struct {
				Revision string `json:"revision"`
			} `json:"fixtures"`
			Validation struct {
				EnvironmentClass    string                          `json:"environment_class"`
				SourceKind          string                          `json:"source_kind"`
				Candidate           releasecontract.DigestReference `json:"candidate"`
				CandidateProvenance releasecontract.DigestReference `json:"candidate_provenance"`
			} `json:"validation"`
			Production struct {
				EnvironmentClass string `json:"environment_class"`
				SourceKind       string `json:"source_kind"`
				ReleasedRecord   any    `json:"released_record"`
			} `json:"production"`
		} `json:"consumer_resolution"`
		Pilot struct {
			Release                   string                          `json:"release"`
			Status                    string                          `json:"status"`
			Candidate                 releasecontract.DigestReference `json:"candidate"`
			CandidateProvenance       releasecontract.DigestReference `json:"candidate_provenance"`
			SystemTestEvidence        any                             `json:"system_test_evidence"`
			ReleasedEnvelope          any                             `json:"released_envelope"`
			AllowedEnvironmentClasses []string                        `json:"allowed_environment_classes"`
			ProductionEligible        bool                            `json:"production_eligible"`
		} `json:"pilot"`
	} `json:"destination_contract"`
	LegacyCompatibility struct {
		RecordInventory               string `json:"record_inventory"`
		Disposition                   string `json:"disposition"`
		ActivePromotionOwner          bool   `json:"active_promotion_owner"`
		CanonicalForNewServerReleases bool   `json:"canonical_for_new_server_releases"`
	} `json:"legacy_compatibility"`
	RetainedTooling []struct {
		SourcePath string `json:"source_path"`
	} `json:"retained_tooling"`
	CutoverGates []struct {
		ID       string `json:"id"`
		Status   string `json:"status"`
		Evidence []any  `json:"evidence"`
	} `json:"cutover_gates"`
	Completion struct {
		Status               string `json:"status"`
		ProductionAuthorized bool   `json:"production_authorized"`
	} `json:"completion"`
}

func TestReleaseContractV1SchemasAcceptExactFixtures(t *testing.T) {
	compiler := releaseSchemaCompiler(t)
	fixtures := map[string]string{
		"candidate.schema.json":             "candidate.json",
		"candidate-provenance.schema.json":  "candidate-provenance.json",
		"system-test-evidence.schema.json":  "system-test-evidence.json",
		"promotion-request.schema.json":     "promotion-request.json",
		"released-envelope.schema.json":     "released-envelope.json",
		"resolution.schema.json":            "validation-resolution.json",
		"resolution.schema.json#production": "production-resolution.json",
	}
	for schemaName, fixtureName := range fixtures {
		t.Run(fixtureName, func(t *testing.T) {
			compileName := strings.TrimSuffix(schemaName, "#production")
			schema, err := compiler.Compile("https://endlessnet.ru/contracts/release/v1/" + compileName)
			if err != nil {
				t.Fatal(err)
			}
			var document any
			decodeFixture(t, fixtureName, &document)
			if err := schema.Validate(document); err != nil {
				t.Fatalf("%s rejected by %s: %v", fixtureName, compileName, err)
			}
		})
	}
}

func TestServerReleaseMigrationInventoryIsExactAndIncomplete(t *testing.T) {
	compiler := releaseSchemaCompiler(t)
	schema, err := compiler.Compile("https://endlessnet.ru/contracts/release-migration/v1/inventory.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile("../release/migration/v1/inventory.json")
	if err != nil {
		t.Fatal(err)
	}
	var document any
	if err := releasecontract.DecodeStrict(raw, &document); err != nil {
		t.Fatal(err)
	}
	if err := schema.Validate(document); err != nil {
		t.Fatal(err)
	}
	var inventory releaseMigrationInventory
	if err := json.Unmarshal(raw, &inventory); err != nil {
		t.Fatal(err)
	}
	if inventory.State != "copy_pending" ||
		inventory.Authority.Repository != "endless-net/architecture" ||
		inventory.Authority.Revision != "a4a4798de03ca93d626dd242b55884aa3d478c67" ||
		inventory.Source.Repository != "endless-net/client-api" ||
		inventory.Source.Revision != "b1fe788fc2f0dd1772a2a6a5dfe2759e83c1d249" ||
		inventory.Source.Role != "frozen_migration_source" ||
		inventory.Target.Repository != "endless-net/releases" ||
		inventory.Target.BaselineRevision != "51775a764544b8aad44a9fc44a9e67da543034cf" ||
		inventory.Completion.Status != "not_complete" ||
		inventory.Completion.ProductionAuthorized {
		t.Fatalf("migration inventory overstates cutover: %+v", inventory)
	}
	for _, gate := range inventory.CutoverGates {
		if gate.Status != "pending" || gate.Evidence != nil {
			t.Fatalf("migration gate unexpectedly completed: %+v", gate)
		}
	}

	type recordIdentity struct {
		target string
		digest string
	}
	records := make(map[string]recordIdentity, len(inventory.Records))
	targets := make(map[string]bool, len(inventory.Records))
	for _, record := range inventory.Records {
		if _, duplicate := records[record.SourcePath]; duplicate || targets[record.TargetPath] {
			t.Fatalf("duplicate migration path: %+v", record)
		}
		records[record.SourcePath] = recordIdentity{target: record.TargetPath, digest: record.SHA256}
		targets[record.TargetPath] = true
	}

	expected := map[string]bool{
		"release/manifest.schema.json": false,
		"release/evidence.schema.json": false,
	}
	for _, directory := range []string{
		"../release/candidates",
		"../release/evidence",
		"../release/schemas/v1",
		"../release/fixtures/v1",
	} {
		entries, err := os.ReadDir(directory)
		if err != nil {
			t.Fatal(err)
		}
		for _, entry := range entries {
			if entry.IsDir() {
				t.Fatalf("unexpected nested legacy source directory %s", filepath.Join(directory, entry.Name()))
			}
			repositoryPath := strings.TrimPrefix(filepath.ToSlash(filepath.Join(directory, entry.Name())), "../")
			expected[repositoryPath] = false
		}
	}
	if len(records) != len(expected) {
		t.Fatalf("migration inventory has %d records, want %d", len(records), len(expected))
	}
	for path := range expected {
		record, ok := records[path]
		if !ok {
			t.Errorf("migration inventory is missing %s", path)
			continue
		}
		contents, err := os.ReadFile("../" + path)
		if err != nil {
			t.Fatal(err)
		}
		if digest := releasecontract.Digest(contents); digest != record.digest {
			t.Errorf("migration digest for %s = %s, want %s", path, record.digest, digest)
		}
		if target := strings.TrimPrefix(path, "release/"); record.target != target {
			t.Errorf("migration target for %s = %s, want %s", path, record.target, target)
		}
	}
}

func TestServerReleaseMigrationInventoryV2PointsAtReleasesAndKeepsCutoverIncomplete(t *testing.T) {
	compiler := releaseSchemaCompiler(t)
	schema, err := compiler.Compile("https://endlessnet.ru/contracts/release-migration/v2/inventory.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile("../release/migration/v2/inventory.json")
	if err != nil {
		t.Fatal(err)
	}
	var document any
	if err := releasecontract.DecodeStrict(raw, &document); err != nil {
		t.Fatal(err)
	}
	if err := schema.Validate(document); err != nil {
		t.Fatal(err)
	}

	var inventory releaseMigrationInventoryV2
	if err := json.Unmarshal(raw, &inventory); err != nil {
		t.Fatal(err)
	}
	const releasesRevision = "89e6129dd7304a05bb2b7f18c771d776058b3dcc"
	if inventory.State != "destination_implemented_consumer_cutover_pending" ||
		inventory.Source.Repository != "endless-net/client-api" ||
		inventory.Source.Revision != "b1fe788fc2f0dd1772a2a6a5dfe2759e83c1d249" ||
		inventory.Source.OwnershipStopRevision != "70848029f2460b1edd18fa3bf17f5f0a37a4e108" ||
		inventory.Source.Role != "frozen_legacy_compatibility_source" ||
		inventory.Target.Repository != "endless-net/releases" ||
		inventory.Target.BaselineRevision != "51775a764544b8aad44a9fc44a9e67da543034cf" ||
		inventory.Target.ImplementationRevision != releasesRevision ||
		inventory.Target.Role != "server_release_control_owner" ||
		inventory.DestinationContract.Revision != releasesRevision ||
		inventory.Completion.Status != "not_complete" || inventory.Completion.ProductionAuthorized {
		t.Fatalf("migration inventory overstates or misdirects cutover: %+v", inventory)
	}

	previousRaw, err := os.ReadFile("../" + inventory.PreviousInventory.Path)
	if err != nil {
		t.Fatal(err)
	}
	if inventory.PreviousInventory.Path != "release/migration/v1/inventory.json" ||
		inventory.PreviousInventory.SHA256 != releasecontract.Digest(previousRaw) ||
		inventory.PreviousInventory.Disposition != "preserved_historical_handoff" {
		t.Fatalf("historical handoff was not preserved exactly: %+v", inventory.PreviousInventory)
	}

	records := make(map[string]string, len(inventory.DestinationContract.Records))
	for _, record := range inventory.DestinationContract.Records {
		if _, duplicate := records[record.Path]; duplicate {
			t.Fatalf("duplicate Releases implementation record %q", record.Path)
		}
		records[record.Path] = record.SHA256
	}
	if len(records) != 25 {
		t.Fatalf("Releases implementation record count = %d, want 25", len(records))
	}
	criticalRecords := map[string]string{
		".github/workflows/ci.yml":                                 "sha256:1686592a4e39925e2b210e328acdafa9d9b2199e573c3e1efaca8139d15fc7a5",
		".github/workflows/promote.yml":                            "sha256:0d38274423c1e6b40ef0cb061f2fd4bc11a7a64da199ad26e9ad38297f975a64",
		"internal/releasecontract/contract.go":                     "sha256:d3ca6b7dc0fef578313b728baf964a76b40f223656812f1ee91d2ab63aa59b5e",
		"internal/releasecontract/contract_test.go":                "sha256:c2dbaa6bc26f5566353a431c20351737d4c74e6a56f5a7687e8a0a63defade7d",
		"schemas/v1/common.schema.json":                            "sha256:6bb1cf132c610b2f42b91362ea9cbb23c3a5ce75f7a1ab9f3b9c0a21ee549cbd",
		"fixtures/v1/validation-resolution.json":                   "sha256:e5d3ad3cbb1f174605663a1cc563ec082445a7a470e934a3d15b991097d761fe",
		"fixtures/v1/production-resolution.json":                   "sha256:474a61aa748b508e7f55c7097c3399129f4682a87d829c7ed2e139036d6eeb5d",
		"candidates/gateway-23121de-d025-pilot-rc1.json":           "sha256:fc037453d1b41f350556b409c709070cc54c9f2c09dff5638d3c93d348eb31fb",
		"candidate-provenance/gateway-23121de-d025-pilot-rc1.json": "sha256:3bfbb55386c8fc30775ac94f2ea84dd77897ab7e7c445c166669a97ba13fd01e",
	}
	for path, digest := range criticalRecords {
		if records[path] != digest {
			t.Errorf("Releases record %s digest = %s, want %s", path, records[path], digest)
		}
	}

	capabilities := make(map[string]bool, len(inventory.DestinationContract.Capabilities))
	for _, capability := range inventory.DestinationContract.Capabilities {
		if capability.Status != "verified" || capabilities[capability.ID] {
			t.Fatalf("invalid or duplicate destination capability: %+v", capability)
		}
		capabilities[capability.ID] = true
		for _, path := range capability.EvidencePaths {
			if _, ok := records[path]; !ok {
				t.Errorf("capability %s cites unpinned path %s", capability.ID, path)
			}
		}
	}
	for _, id := range []string{
		"v1_schemas",
		"semantic_tooling",
		"exact_fixtures",
		"append_only_gate",
		"actor_recording_protected_promotion",
		"pilot_candidate",
	} {
		if !capabilities[id] {
			t.Errorf("missing verified Releases capability %q", id)
		}
	}

	resolution := inventory.DestinationContract.ConsumerResolution
	pilot := inventory.DestinationContract.Pilot
	if resolution.Schemas.Revision != releasesRevision || resolution.Schemas.CopyIntoConsumers ||
		resolution.Fixtures.Revision != releasesRevision ||
		resolution.Validation.EnvironmentClass != "validation" || resolution.Validation.SourceKind != "candidate" ||
		resolution.Production.EnvironmentClass != "production" || resolution.Production.SourceKind != "released" ||
		resolution.Production.ReleasedRecord != nil ||
		pilot.Release != "gateway-23121de-d025-pilot-rc1" || pilot.Status != "candidate" ||
		pilot.Candidate != resolution.Validation.Candidate ||
		pilot.CandidateProvenance != resolution.Validation.CandidateProvenance ||
		pilot.SystemTestEvidence != nil || pilot.ReleasedEnvelope != nil || pilot.ProductionEligible ||
		!reflect.DeepEqual(pilot.AllowedEnvironmentClasses, []string{"validation"}) {
		t.Fatalf("consumer resolution policy permits an unproved production input: %+v", resolution)
	}

	if inventory.LegacyCompatibility.RecordInventory != "release/migration/v1/inventory.json" ||
		inventory.LegacyCompatibility.Disposition != "preserve_frozen_in_place" ||
		inventory.LegacyCompatibility.ActivePromotionOwner ||
		inventory.LegacyCompatibility.CanonicalForNewServerReleases {
		t.Fatalf("legacy Client API ownership was not stopped safely: %+v", inventory.LegacyCompatibility)
	}
	for _, tool := range inventory.RetainedTooling {
		if _, err := os.Stat("../" + tool.SourcePath); err != nil {
			t.Errorf("retained compatibility gate %s: %v", tool.SourcePath, err)
		}
	}

	wantGates := map[string]string{
		"releases_destination_contract":       "verified",
		"releases_append_only_promotion":      "verified",
		"client_api_active_promotion_stopped": "verified",
		"system_tests_consumer":               "pending",
		"infrastructure_consumer":             "pending",
		"client_api_tooling_retirement":       "pending",
	}
	for _, gate := range inventory.CutoverGates {
		want, ok := wantGates[gate.ID]
		if !ok || gate.Status != want {
			t.Fatalf("unexpected migration gate: %+v", gate)
		}
		if (gate.Status == "verified" && len(gate.Evidence) == 0) ||
			(gate.Status == "pending" && len(gate.Evidence) != 0) {
			t.Fatalf("migration gate has inconsistent evidence: %+v", gate)
		}
		delete(wantGates, gate.ID)
	}
	if len(wantGates) != 0 {
		t.Fatalf("missing migration gates: %+v", wantGates)
	}
}

func TestD025ComponentReleaseContractDoesNotRequireServerSet(t *testing.T) {
	compiler := releaseSchemaCompiler(t)
	recordSchema, err := compiler.Compile("https://endlessnet.ru/contracts/release-migration/v3/component-release.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	inventorySchema, err := compiler.Compile("https://endlessnet.ru/contracts/release-migration/v3/inventory.schema.json")
	if err != nil {
		t.Fatal(err)
	}

	var inventory any
	decodePath(t, "../release/migration/v3/inventory.json", &inventory)
	if err := inventorySchema.Validate(inventory); err != nil {
		t.Fatalf("D-025 inventory rejected: %v", err)
	}

	var candidateDocument any
	candidateRaw := decodePath(t, "../release/migration/v3/candidate.json", &candidateDocument)
	if err := recordSchema.Validate(candidateDocument); err != nil {
		t.Fatalf("single-component candidate rejected: %v", err)
	}
	var candidate releasecontract.ComponentCandidate
	if err := releasecontract.DecodeStrict(candidateRaw, &candidate); err != nil {
		t.Fatal(err)
	}
	if err := releasecontract.ValidateComponentCandidate(candidate); err != nil {
		t.Fatalf("single-component candidate rejected semantically: %v", err)
	}

	var releasedDocument any
	releasedRaw := decodePath(t, "../release/migration/v3/released.json", &releasedDocument)
	if err := recordSchema.Validate(releasedDocument); err != nil {
		t.Fatalf("released component rejected: %v", err)
	}
	var released releasecontract.ReleasedComponent
	if err := releasecontract.DecodeStrict(releasedRaw, &released); err != nil {
		t.Fatal(err)
	}
	if err := releasecontract.ValidateReleasedComponent(released, candidate, candidateRaw); err != nil {
		t.Fatalf("released component rejected semantically: %v", err)
	}

	var signalDocument any
	signalRaw := decodePath(t, "../release/migration/v3/infrastructure-signal.json", &signalDocument)
	if err := recordSchema.Validate(signalDocument); err != nil {
		t.Fatalf("Infrastructure signal rejected: %v", err)
	}
	var signal releasecontract.InfrastructureSignal
	if err := releasecontract.DecodeStrict(signalRaw, &signal); err != nil {
		t.Fatal(err)
	}
	if err := releasecontract.ValidateInfrastructureSignal(signal, released, releasedRaw); err != nil {
		t.Fatalf("Infrastructure signal rejected semantically: %v", err)
	}

	t.Run("released record requires every affected edge", func(t *testing.T) {
		drifted := released
		drifted.CompatibilityEvidence = nil
		if err := releasecontract.ValidateReleasedComponent(drifted, candidate, candidateRaw); err == nil {
			t.Fatal("released component without affected compatibility evidence was accepted")
		}
	})

	t.Run("signal cannot substitute a candidate or override the component", func(t *testing.T) {
		drifted := signal
		drifted.Component = "identity"
		if err := releasecontract.ValidateInfrastructureSignal(drifted, released, releasedRaw); err == nil {
			t.Fatal("Infrastructure signal with a component override was accepted")
		}
	})
}

func TestPromotionCopiesTheExactTestedCandidate(t *testing.T) {
	fixture := loadReleaseContractFixture(t)
	actual, err := releasecontract.BuildReleasedEnvelope(
		fixture.candidate,
		fixture.candidateRaw,
		fixture.provenanceRaw,
		fixture.systemTestRaw,
		fixture.provenance,
		fixture.systemTest,
		fixture.request,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(actual, fixture.envelope) {
		t.Fatalf("promoted envelope differs from exact fixture\nactual: %#v\nfixture: %#v", actual, fixture.envelope)
	}
	if err := releasecontract.ValidateReleasedEnvelope(
		fixture.envelope,
		fixture.candidate,
		fixture.candidateRaw,
		fixture.provenanceRaw,
		fixture.systemTestRaw,
		fixture.provenance,
		fixture.systemTest,
	); err != nil {
		t.Fatal(err)
	}
}

func TestReleaseContractRejectsDriftTagsAndMissingProvenance(t *testing.T) {
	fixture := loadReleaseContractFixture(t)

	t.Run("component drift", func(t *testing.T) {
		drifted := fixture.envelope
		drifted.Resolved.Components = append([]releasecontract.Component(nil), drifted.Resolved.Components...)
		drifted.Resolved.Components[0].GitCommit = strings.Repeat("f", 40)
		if err := releasecontract.ValidateReleasedEnvelope(
			drifted,
			fixture.candidate,
			fixture.candidateRaw,
			fixture.provenanceRaw,
			fixture.systemTestRaw,
			fixture.provenance,
			fixture.systemTest,
		); err == nil {
			t.Fatal("released component drift was accepted")
		}
	})

	t.Run("mutable tag", func(t *testing.T) {
		candidate := fixture.candidate
		candidate.Components = append([]releasecontract.Component(nil), candidate.Components...)
		mutable := "ghcr.io/endless-net/gateway:latest"
		candidate.Components[0].Image = &mutable
		if err := releasecontract.ValidateCandidate(candidate); err == nil {
			t.Fatal("mutable candidate artifact tag was accepted")
		}
	})

	t.Run("missing component provenance", func(t *testing.T) {
		provenance := fixture.provenance
		provenance.Components = provenance.Components[:1]
		if err := releasecontract.ValidateCandidateProvenance(
			fixture.candidate,
			releasecontract.Digest(fixture.candidateRaw),
			provenance,
		); err == nil {
			t.Fatal("incomplete component provenance was accepted")
		}
	})

	t.Run("missing promotion provenance", func(t *testing.T) {
		request := fixture.request
		request.Promotion = releasecontract.PromotionProvenance{}
		if _, err := releasecontract.BuildReleasedEnvelope(
			fixture.candidate,
			fixture.candidateRaw,
			fixture.provenanceRaw,
			fixture.systemTestRaw,
			fixture.provenance,
			fixture.systemTest,
			request,
		); err == nil {
			t.Fatal("missing promotion provenance was accepted")
		}
	})
}

func TestResolutionSeparatesValidationAndProductionInputs(t *testing.T) {
	fixture := loadReleaseContractFixture(t)
	var wantValidation, wantProduction releasecontract.Resolution
	decodeFixture(t, "validation-resolution.json", &wantValidation)
	decodeFixture(t, "production-resolution.json", &wantProduction)

	validation, err := releasecontract.ResolveCandidate(
		fixture.candidate,
		fixture.candidateRaw,
		fixture.provenanceRaw,
		fixture.provenance,
		fixture.request.Candidate,
		fixture.request.CandidateProvenance,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(validation, wantValidation) {
		t.Fatalf("validation resolution drifted\nactual: %#v\nfixture: %#v", validation, wantValidation)
	}

	envelopeReference := wantProduction.Source.Record
	production, err := releasecontract.ResolveProduction(fixture.envelope, fixture.envelopeRaw, envelopeReference)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(production, wantProduction) {
		t.Fatalf("production resolution drifted\nactual: %#v\nfixture: %#v", production, wantProduction)
	}

	candidateAsEnvelope := releasecontract.ReleasedEnvelope{
		SchemaVersion: 1,
		Release:       fixture.candidate.Release,
		Status:        "candidate",
	}
	if _, err := releasecontract.ResolveProduction(candidateAsEnvelope, fixture.candidateRaw, fixture.request.Candidate); err == nil {
		t.Fatal("production resolver accepted a candidate")
	}
}

func TestSchemaRejectsEnvironmentAndProvenanceViolations(t *testing.T) {
	compiler := releaseSchemaCompiler(t)
	resolutionSchema, err := compiler.Compile("https://endlessnet.ru/contracts/release/v1/resolution.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	releasedSchema, err := compiler.Compile("https://endlessnet.ru/contracts/release/v1/released-envelope.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	candidateSchema, err := compiler.Compile("https://endlessnet.ru/contracts/release/v1/candidate.schema.json")
	if err != nil {
		t.Fatal(err)
	}

	var validation map[string]any
	decodeFixture(t, "validation-resolution.json", &validation)
	validation["environment_class"] = "production"
	if err := resolutionSchema.Validate(validation); err == nil {
		t.Fatal("production resolution schema accepted a candidate source")
	}

	var envelope map[string]any
	decodeFixture(t, "released-envelope.json", &envelope)
	delete(envelope, "provenance")
	if err := releasedSchema.Validate(envelope); err == nil {
		t.Fatal("released schema accepted missing provenance")
	}

	var candidate map[string]any
	decodeFixture(t, "candidate.json", &candidate)
	components := candidate["components"].([]any)
	components[0].(map[string]any)["image"] = "ghcr.io/endless-net/gateway:latest"
	if err := candidateSchema.Validate(candidate); err == nil {
		t.Fatal("candidate schema accepted a mutable artifact tag")
	}
}

func TestFrozenLegacyServerReleaseSourceCannotChange(t *testing.T) {
	for _, changes := range []string{
		"M\trelease/candidates/example.json",
		"A\trelease/candidates/new.json",
		"D\trelease/releases/example.json",
		"R100\trelease/releases/old.json\trelease/releases/new.json",
		"M\trelease/candidate-provenance/example.json",
		"M\trelease/system-test-evidence/example.json",
		"A\trelease/schemas/v2/manifest.schema.json",
		"M\trelease/fixtures/v1/candidate.json",
		"M\trelease/manifest.schema.json",
	} {
		if err := releasecontract.ValidateImmutableChanges(changes); err == nil {
			t.Fatalf("immutable change was accepted: %q", changes)
		}
	}
	if err := releasecontract.ValidateImmutableChanges("A\trelease/migration/v1/inventory.json\nM\trelease/README.md"); err != nil {
		t.Fatalf("migration metadata change was rejected: %v", err)
	}

	path := filepath.Join(t.TempDir(), "released.json")
	if err := releasecontract.WriteNewJSON(path, map[string]int{"schema_version": 1}); err != nil {
		t.Fatal(err)
	}
	if err := releasecontract.WriteNewJSON(path, map[string]int{"schema_version": 2}); err == nil {
		t.Fatal("promotion output overwrote an existing released record")
	}
}

func TestCheckedInReleasedEnvelopesBindTheirLocalRecords(t *testing.T) {
	files, err := filepath.Glob("../release/releases/*.json")
	if err != nil {
		t.Fatal(err)
	}
	for _, envelopePath := range files {
		release := strings.TrimSuffix(filepath.Base(envelopePath), ".json")
		t.Run(release, func(t *testing.T) {
			var envelope releasecontract.ReleasedEnvelope
			decodePath(t, envelopePath, &envelope)
			var candidate releasecontract.Candidate
			candidateRaw := decodePath(t, filepath.Join("../release/candidates", release+".json"), &candidate)
			var provenance releasecontract.CandidateProvenance
			provenanceRaw := decodePath(t, filepath.Join("../release/candidate-provenance", release+".json"), &provenance)
			var evidence releasecontract.SystemTestEvidence
			systemTestRaw := decodePath(t, filepath.Join("../release/system-test-evidence", release+".json"), &evidence)
			if err := releasecontract.ValidateReleasedEnvelope(
				envelope,
				candidate,
				candidateRaw,
				provenanceRaw,
				systemTestRaw,
				provenance,
				evidence,
			); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func loadReleaseContractFixture(t *testing.T) releaseContractFixture {
	t.Helper()
	fixture := releaseContractFixture{}
	fixture.candidateRaw = decodeFixture(t, "candidate.json", &fixture.candidate)
	fixture.provenanceRaw = decodeFixture(t, "candidate-provenance.json", &fixture.provenance)
	fixture.systemTestRaw = decodeFixture(t, "system-test-evidence.json", &fixture.systemTest)
	decodeFixture(t, "promotion-request.json", &fixture.request)
	fixture.envelopeRaw = decodeFixture(t, "released-envelope.json", &fixture.envelope)
	return fixture
}

func decodeFixture(t *testing.T, name string, value any) []byte {
	t.Helper()
	return decodePath(t, filepath.Join(releaseFixtureDirectory, name), value)
}

func decodePath(t *testing.T, path string, value any) []byte {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := releasecontract.DecodeStrict(raw, value); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
	return raw
}

func releaseSchemaCompiler(t *testing.T) *jsonschema.Compiler {
	t.Helper()
	compiler := jsonschema.NewCompiler()
	resources := map[string]string{
		"https://endlessnet.ru/contracts/release-manifest.schema.json":                       "../release/manifest.schema.json",
		"https://endlessnet.ru/contracts/release/v1/common.schema.json":                      "../release/schemas/v1/common.schema.json",
		"https://endlessnet.ru/contracts/release/v1/candidate.schema.json":                   "../release/schemas/v1/candidate.schema.json",
		"https://endlessnet.ru/contracts/release/v1/candidate-provenance.schema.json":        "../release/schemas/v1/candidate-provenance.schema.json",
		"https://endlessnet.ru/contracts/release/v1/system-test-evidence.schema.json":        "../release/schemas/v1/system-test-evidence.schema.json",
		"https://endlessnet.ru/contracts/release/v1/promotion-request.schema.json":           "../release/schemas/v1/promotion-request.schema.json",
		"https://endlessnet.ru/contracts/release/v1/released-envelope.schema.json":           "../release/schemas/v1/released-envelope.schema.json",
		"https://endlessnet.ru/contracts/release/v1/resolution.schema.json":                  "../release/schemas/v1/resolution.schema.json",
		"https://endlessnet.ru/contracts/release-migration/v1/inventory.schema.json":         "../release/migration/v1/inventory.schema.json",
		"https://endlessnet.ru/contracts/release-migration/v2/inventory.schema.json":         "../release/migration/v2/inventory.schema.json",
		"https://endlessnet.ru/contracts/release-migration/v3/inventory.schema.json":         "../release/migration/v3/inventory.schema.json",
		"https://endlessnet.ru/contracts/release-migration/v3/component-release.schema.json": "../release/migration/v3/component-release.schema.json",
	}
	for identifier, path := range resources {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		document, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
		if err != nil {
			t.Fatalf("decode schema %s: %v", path, err)
		}
		if err := compiler.AddResource(identifier, document); err != nil {
			t.Fatalf("add schema %s: %v", path, err)
		}
	}
	return compiler
}
