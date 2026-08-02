package systemtests

import (
	"bytes"
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

func TestImmutableReleaseRecordsCannotBeEdited(t *testing.T) {
	for _, changes := range []string{
		"M\trelease/candidates/example.json",
		"D\trelease/releases/example.json",
		"R100\trelease/releases/old.json\trelease/releases/new.json",
		"M\trelease/candidate-provenance/example.json",
		"M\trelease/system-test-evidence/example.json",
	} {
		if err := releasecontract.ValidateImmutableChanges(changes); err == nil {
			t.Fatalf("immutable change was accepted: %q", changes)
		}
	}
	if err := releasecontract.ValidateImmutableChanges("A\trelease/releases/new.json\nM\trelease/README.md"); err != nil {
		t.Fatalf("new released record was rejected: %v", err)
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
		"https://endlessnet.ru/contracts/release-manifest.schema.json":                "../release/manifest.schema.json",
		"https://endlessnet.ru/contracts/release/v1/common.schema.json":               "../release/schemas/v1/common.schema.json",
		"https://endlessnet.ru/contracts/release/v1/candidate.schema.json":            "../release/schemas/v1/candidate.schema.json",
		"https://endlessnet.ru/contracts/release/v1/candidate-provenance.schema.json": "../release/schemas/v1/candidate-provenance.schema.json",
		"https://endlessnet.ru/contracts/release/v1/system-test-evidence.schema.json": "../release/schemas/v1/system-test-evidence.schema.json",
		"https://endlessnet.ru/contracts/release/v1/promotion-request.schema.json":    "../release/schemas/v1/promotion-request.schema.json",
		"https://endlessnet.ru/contracts/release/v1/released-envelope.schema.json":    "../release/schemas/v1/released-envelope.schema.json",
		"https://endlessnet.ru/contracts/release/v1/resolution.schema.json":           "../release/schemas/v1/resolution.schema.json",
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
