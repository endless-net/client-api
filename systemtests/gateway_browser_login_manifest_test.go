package systemtests

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"testing"
)

const (
	gatewayBrowserLoginContractName       = "gateway-browser-login"
	gatewayBrowserLoginArchitectureCommit = "62175d5e97e3a5a57dcb0f2ab2c377c5eb7cd4ac"
	gatewayBrowserLoginRelease            = "gateway-browser-login-v2-rc1"
	gatewayBrowserLoginManifestPath       = "../release/candidates/gateway-browser-login-v2-rc1.json"
	gatewayBrowserLoginEvidencePath       = "../release/evidence/gateway-browser-login-v2-rc1.json"
	systemTestsF4ffRevision               = "f4ff29a13d973c4067c0a3787355bc197dd8c40a"
)

type expectedComponent struct {
	repository string
	commit     string
	image      *string
}

var gatewayBrowserLoginComponents = map[string]expectedComponent{
	"gateway": {
		repository: "endless-net/gateway",
		commit:     "0ab32cc3df4169a58bb60eea4a29c9545a8f72f7",
		image:      stringPointer("https://github.com/endless-net/gateway/actions/runs/30739618344/artifacts/8830831857@sha256:a46bdbb94ebc4c23fca60c5f96f894f34cc2fe7c72e93038fc71a73d989e6858"),
	},
	"admin": {
		repository: "endless-net/admin",
		commit:     "cca0118d7d660bd48e20dcaf1367fdc2a803ea55",
		image:      stringPointer("ghcr.io/endless-net/endlessnet-admin-web@sha256:b6a37f40ef7b71b90dfc4f5167d6f23d95d3d53b6dd0dafb8b139f5bb47ee651"),
	},
	"identity": {
		repository: "endless-net/identity",
		commit:     "64d768d3ad5cc41e6f5c875747e6a7bd44d752d1",
		image:      stringPointer("ghcr.io/endless-net/identity@sha256:48df9459c0d8da6e17b1fcb5890e7424a10308979e449b5f25abfff44b246ede"),
	},
	"billing": {
		repository: "endless-net/billing",
		commit:     "fffd30550a6eb5dfd62c5b8978bc9699a20f6cc5",
		image:      stringPointer("ghcr.io/endless-net/billing@sha256:6cbe6418b8e08b7b84a5161321779cdaa9d319b508b5e17cb16eda275a12ff53"),
	},
	"coordinator": {
		repository: "endless-net/coordinator",
		commit:     "8c939bd22b6b5e6dbff6b35f058135ed7ead491c",
		image:      stringPointer("ghcr.io/endless-net/coordinator@sha256:f69c3610e0bd43cd192e07a16a74ca062e0bed607a3d96f39fbb128e805f7cf1"),
	},
	"signing": {
		repository: "endless-net/signing",
		commit:     "9866a6aff84f98de34010bcf3ea9e33fe90425fb",
		image:      stringPointer("ghcr.io/endless-net/signing@sha256:46a2918a5a6ec4daf26037c1b6774457e1a1b13ada3da9fac3d0bbc44461360f"),
	},
	"management": {
		repository: "endless-net/management",
		commit:     "4ab0a61bf066ad35a1d1dbc5c4e637f461ebf333",
		image:      stringPointer("https://github.com/endless-net/management/actions/runs/30715074238/artifacts/8823085344@sha256:9ada3efeb8536943d6d5fec24ed29d0322241b87deb3fd4309ed74efc716d33f"),
	},
	"relay": {
		repository: "endless-net/relay",
		commit:     "5b9c3237a346a9b8c02dd44b712850ec230f1bdb",
		image:      stringPointer("ghcr.io/endless-net/relay-public@sha256:9e62284925ae501bc12fb48b76aa39275447946bd13daa9992e28b62737c1ab3"),
	},
	"stun": {
		repository: "endless-net/stun",
		commit:     "7a88454004103e44b3a0fba85da1396e6df992b6",
		image:      stringPointer("ghcr.io/endless-net/stun-public@sha256:3251da018cdcc0213abaa8987023cad127172dcc27ff3429b3f1ddd1be2406f3"),
	},
	"client": {
		repository: "endless-net/client",
		commit:     "7ca528475b794eead1e3d482690cc2437c6d854a",
		image:      nil,
	},
	"system-tests-oidc-fixture": {
		repository: "endless-net/system-tests",
		commit:     "f4ff29a13d973c4067c0a3787355bc197dd8c40a",
		image:      stringPointer("ghcr.io/endless-net/system-tests-oidc-fixture@sha256:a9fc121fbf5d7437c4abf3091f7a24a45b53d7ebe3bffdb98ff1a4dbe3fc6f03"),
	},
}

type releaseEvidence struct {
	Schema        string `json:"$schema"`
	SchemaVersion int    `json:"schema_version"`
	Release       string `json:"release"`
	Manifest      struct {
		Path   string `json:"path"`
		SHA256 string `json:"sha256"`
	} `json:"manifest"`
	Contract releaseContract `json:"contract"`
	Gateway  struct {
		Repository      string          `json:"repository"`
		SourceCommit    string          `json:"source_commit"`
		PublisherCommit string          `json:"publisher_commit"`
		CIRun           int64           `json:"ci_run"`
		PublicationRun  int64           `json:"publication_run"`
		Artifact        actionsArtifact `json:"artifact"`
		ArchiveDigest   string          `json:"archive_digest"`
		BinarySHA256    string          `json:"binary_sha256"`
		Provenance      provenance      `json:"provenance"`
	} `json:"gateway"`
	Admin struct {
		Repository         string          `json:"repository"`
		SourceCommit       string          `json:"source_commit"`
		PublicationRun     int64           `json:"publication_run"`
		Image              string          `json:"image"`
		ArchiveSHA256      string          `json:"archive_sha256"`
		TestedArtifact     actionsArtifact `json:"tested_artifact"`
		DescriptorArtifact actionsArtifact `json:"descriptor_artifact"`
		Provenance         provenance      `json:"provenance"`
	} `json:"admin"`
	Identity struct {
		Repository              string          `json:"repository"`
		SourceCommit            string          `json:"source_commit"`
		PublisherCommit         string          `json:"publisher_commit"`
		SourceCIRun             int64           `json:"source_ci_run"`
		PublisherCIRun          int64           `json:"publisher_ci_run"`
		PublicationRun          int64           `json:"publication_run"`
		Image                   string          `json:"image"`
		ArchiveDigest           string          `json:"archive_digest"`
		BuildArtifact           actionsArtifact `json:"build_artifact"`
		PublicationArtifact     actionsArtifact `json:"publication_artifact"`
		PublicationRecordSHA256 string          `json:"publication_record_sha256"`
		Provenance              provenance      `json:"provenance"`
	} `json:"identity"`
	SystemTests struct {
		Repository                 string          `json:"repository"`
		SourceCommit               string          `json:"source_commit"`
		PublisherCommit            string          `json:"publisher_commit"`
		CIRun                      int64           `json:"ci_run"`
		PublicationRun             int64           `json:"publication_run"`
		FixtureImage               string          `json:"fixture_image"`
		FixtureBinarySHA256        string          `json:"fixture_binary_sha256"`
		FixturePublicationArtifact actionsArtifact `json:"fixture_publication_artifact"`
		Provenance                 provenance      `json:"provenance"`
	} `json:"system_tests"`
}

type actionsArtifact struct {
	ID     int64  `json:"id"`
	Digest string `json:"digest"`
}

type provenance struct {
	Status                    string `json:"status"`
	RekorIndex                int64  `json:"rekor_index,omitempty"`
	AttestationManifestDigest string `json:"attestation_manifest_digest,omitempty"`
	ProvenanceDigest          string `json:"provenance_digest,omitempty"`
}

// systemTestsF4ffReleaseManifest is the strict compatibility-manifest decode
// shape used by cmd/manifestcheck at systemTestsF4ffRevision. Keep this narrow:
// producer validation remains owned by endless-net/system-tests.
// Source: https://github.com/endless-net/system-tests/blob/f4ff29a13d973c4067c0a3787355bc197dd8c40a/cmd/manifestcheck/main.go
type systemTestsF4ffReleaseManifest struct {
	Schema        string             `json:"$schema"`
	SchemaVersion int                `json:"schema_version"`
	Release       string             `json:"release"`
	Status        string             `json:"status"`
	Components    []releaseComponent `json:"components"`
	Modules       []releaseModule    `json:"modules"`
}

func TestGatewayBrowserLoginCandidateMatchesSystemTestsF4ffManifestCheckShape(t *testing.T) {
	raw, err := os.ReadFile(gatewayBrowserLoginManifestPath)
	if err != nil {
		t.Fatal(err)
	}
	var consumerManifest systemTestsF4ffReleaseManifest
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&consumerManifest); err != nil {
		t.Fatalf("System Tests %s manifestcheck rejected candidate: %v", systemTestsF4ffRevision, err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		t.Fatalf("System Tests %s manifestcheck found trailing JSON: %v", systemTestsF4ffRevision, err)
	}
	if consumerManifest.SchemaVersion != 1 || consumerManifest.Release != gatewayBrowserLoginRelease ||
		consumerManifest.Status != "candidate" || consumerManifest.Modules == nil {
		t.Fatalf("System Tests %s manifestcheck received an unsupported header", systemTestsF4ffRevision)
	}
}

func TestGatewayBrowserLoginCandidateAndEvidence(t *testing.T) {
	manifestRaw, err := os.ReadFile(gatewayBrowserLoginManifestPath)
	if err != nil {
		t.Fatal(err)
	}
	var manifest releaseManifest
	if err := json.Unmarshal(manifestRaw, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.Release != gatewayBrowserLoginRelease || manifest.Status != "candidate" {
		t.Fatalf("unexpected candidate header: release=%q status=%q", manifest.Release, manifest.Status)
	}
	if err := validateGatewayBrowserLoginPins(manifest); err != nil {
		t.Fatal(err)
	}

	evidenceRaw, err := os.ReadFile(gatewayBrowserLoginEvidencePath)
	if err != nil {
		t.Fatal(err)
	}
	var evidence releaseEvidence
	decoder := json.NewDecoder(bytes.NewReader(evidenceRaw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&evidence); err != nil {
		t.Fatal(err)
	}
	manifestDigest := sha256.Sum256(manifestRaw)
	expectedManifestDigest := "sha256:" + hex.EncodeToString(manifestDigest[:])
	if evidence.SchemaVersion != 1 || evidence.Release != manifest.Release ||
		evidence.Manifest.Path != "release/candidates/gateway-browser-login-v2-rc1.json" ||
		evidence.Manifest.SHA256 != expectedManifestDigest {
		t.Fatalf("evidence does not bind the candidate: %+v expected digest %s", evidence.Manifest, expectedManifestDigest)
	}
	if err := validateGatewayBrowserLoginEvidence(evidence); err != nil {
		t.Fatal(err)
	}
}

func TestGatewayBrowserLoginRejectsSubstitution(t *testing.T) {
	manifest := releaseManifest{
		Release: gatewayBrowserLoginRelease,
	}
	for name, expected := range gatewayBrowserLoginComponents {
		manifest.Components = append(manifest.Components, releaseComponent{
			Name: name, Repository: expected.repository, GitCommit: expected.commit, Image: expected.image,
		})
	}
	if err := validateGatewayBrowserLoginPins(manifest); err != nil {
		t.Fatalf("valid fixture rejected: %v", err)
	}
	for index := range manifest.Components {
		if manifest.Components[index].Name == "gateway" {
			manifest.Components[index].GitCommit = "main"
			break
		}
	}
	if err := validateGatewayBrowserLoginPins(manifest); err == nil {
		t.Fatal("mismatched or branch-head Gateway commit was accepted")
	}
	for index := range manifest.Components {
		if manifest.Components[index].Name == "gateway" {
			manifest.Components[index].GitCommit = gatewayBrowserLoginComponents["gateway"].commit
			manifest.Components[index].Image = stringPointer("ghcr.io/endless-net/gateway:main")
			break
		}
	}
	if err := validateGatewayBrowserLoginPins(manifest); err == nil {
		t.Fatal("mutable Gateway tag was accepted")
	}
}

func validateGatewayBrowserLoginPins(manifest releaseManifest) error {
	if manifest.Release != gatewayBrowserLoginRelease {
		return nil
	}
	components := make(map[string]releaseComponent, len(manifest.Components))
	for _, component := range manifest.Components {
		components[component.Name] = component
	}
	for name, expected := range gatewayBrowserLoginComponents {
		component, ok := components[name]
		if !ok || component.Repository != expected.repository || component.GitCommit != expected.commit || !equalStrings(component.Image, expected.image) {
			return fmt.Errorf("%s requires exact immutable %s pin", gatewayBrowserLoginContractName, name)
		}
	}
	return nil
}

func validateGatewayBrowserLoginEvidence(evidence releaseEvidence) error {
	digestPattern := regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
	shaPattern := regexp.MustCompile(`^[0-9a-f]{64}$`)
	if evidence.Schema != "../evidence.schema.json" {
		return fmt.Errorf("evidence has a mismatched schema location")
	}
	if evidence.Contract.Name != gatewayBrowserLoginContractName || evidence.Contract.Version != 2 ||
		evidence.Contract.ArchitectureRepository != "endless-net/architecture" ||
		evidence.Contract.ArchitectureCommit != gatewayBrowserLoginArchitectureCommit {
		return fmt.Errorf("evidence has a mismatched architecture contract")
	}
	if evidence.Gateway.Repository != "endless-net/gateway" ||
		evidence.Gateway.SourceCommit != gatewayBrowserLoginComponents["gateway"].commit ||
		evidence.Gateway.PublisherCommit != gatewayBrowserLoginComponents["gateway"].commit ||
		evidence.Gateway.CIRun != 30739588233 || evidence.Gateway.PublicationRun != 30739618344 ||
		evidence.Gateway.Artifact.ID != 8830831857 ||
		evidence.Gateway.Artifact.Digest != "sha256:a46bdbb94ebc4c23fca60c5f96f894f34cc2fe7c72e93038fc71a73d989e6858" ||
		evidence.Gateway.ArchiveDigest != "sha256:d2c85abe141fe89e200cc1fe53c8ba47251f62654d771fa94dfc62d8f730cdcf" ||
		evidence.Gateway.BinarySHA256 != "0fb8b3512e2307695a4dee18ee5aeb705d6f73ca19867c7a33e6b2e40a7a3ddc" ||
		evidence.Gateway.Provenance.Status != "verified" || evidence.Gateway.Provenance.RekorIndex != 2321192030 {
		return fmt.Errorf("evidence has mismatched Gateway provenance")
	}
	if evidence.Admin.Repository != "endless-net/admin" ||
		evidence.Admin.SourceCommit != gatewayBrowserLoginComponents["admin"].commit ||
		evidence.Admin.PublicationRun != 30708281871 ||
		evidence.Admin.Image != *gatewayBrowserLoginComponents["admin"].image ||
		evidence.Admin.ArchiveSHA256 != "32476e4ede4e1a1b875d3fca18aeedc30dc4eeb0ab457e6e79223991c9657402" ||
		evidence.Admin.TestedArtifact.ID != 8821100019 ||
		evidence.Admin.TestedArtifact.Digest != "sha256:f618227d9f0277354bed95a318fc77c932b84b8a4742b10198d793d1e0cc42c2" ||
		evidence.Admin.DescriptorArtifact.ID != 8821101722 ||
		evidence.Admin.DescriptorArtifact.Digest != "sha256:1a90f2b2f098279cf4e1e225588fa059bfc82654a40a73db3132985e84b0efe9" ||
		evidence.Admin.Provenance.Status != "not-produced-private-repository" {
		return fmt.Errorf("evidence has mismatched Admin provenance")
	}
	if evidence.Identity.Repository != "endless-net/identity" ||
		evidence.Identity.SourceCommit != gatewayBrowserLoginComponents["identity"].commit ||
		evidence.Identity.PublisherCommit != "66b88d5b74c208fb47012fe199c65afeef43111b" ||
		evidence.Identity.SourceCIRun != 30715720621 || evidence.Identity.PublisherCIRun != 30717817761 ||
		evidence.Identity.PublicationRun != 30717819869 ||
		evidence.Identity.Image != *gatewayBrowserLoginComponents["identity"].image ||
		evidence.Identity.ArchiveDigest != "sha256:06ea147c9a78b87731a392fe2520c548a9dc566c799c444ce6c54f6445aaf0da" ||
		evidence.Identity.BuildArtifact.ID != 8823912868 ||
		evidence.Identity.BuildArtifact.Digest != "sha256:bfe8ade43d9a283fb0284e05eed4dadb8c9f423582d251c87515af08fb5b06f4" ||
		evidence.Identity.PublicationArtifact.ID != 8823912992 ||
		evidence.Identity.PublicationArtifact.Digest != "sha256:5235cc4d767071eb0b7559a1439a42bb0ad5399cccab248657ffb24c27c7d730" ||
		evidence.Identity.PublicationRecordSHA256 != "4818ae2e71fdd537573d1c5b00b3a1542883784e580a1d6dc2d96218491a0e2a" ||
		evidence.Identity.Provenance.Status != "verified" || evidence.Identity.Provenance.RekorIndex != 2314531454 ||
		evidence.Identity.Provenance.ProvenanceDigest != "sha256:af374e3f33cbf977abc3f7002304e5a20d8502ce677c2c11569dabca3cf73374" {
		return fmt.Errorf("evidence has mismatched Identity provenance")
	}
	if evidence.SystemTests.Repository != "endless-net/system-tests" ||
		evidence.SystemTests.SourceCommit != gatewayBrowserLoginComponents["system-tests-oidc-fixture"].commit ||
		evidence.SystemTests.PublisherCommit != gatewayBrowserLoginComponents["system-tests-oidc-fixture"].commit ||
		evidence.SystemTests.CIRun != 30739665586 || evidence.SystemTests.PublicationRun != 30739698224 ||
		evidence.SystemTests.FixtureImage != *gatewayBrowserLoginComponents["system-tests-oidc-fixture"].image ||
		evidence.SystemTests.FixtureBinarySHA256 != "17a50055c10a3ff932ebb026c4c29070d76a255fdb5a45263fff31ab0437c3c3" ||
		evidence.SystemTests.FixturePublicationArtifact.ID != 8830853301 ||
		evidence.SystemTests.Provenance.AttestationManifestDigest != "sha256:4cfbf3231e0ced3daa1bd3a795526523c377aba39201333d8f355e7a4e24b824" ||
		evidence.SystemTests.FixturePublicationArtifact.Digest != "sha256:c0d1cd057aa4e8bad3f93102d76e58432655c5605a86231e53fbe01138a66ff0" ||
		evidence.SystemTests.Provenance.Status != "verified" {
		return fmt.Errorf("evidence has mismatched System Tests fixture provenance")
	}
	for _, digest := range []string{
		evidence.Manifest.SHA256, evidence.Gateway.Artifact.Digest, evidence.Gateway.ArchiveDigest,
		evidence.Admin.TestedArtifact.Digest, evidence.Admin.DescriptorArtifact.Digest,
		evidence.Identity.ArchiveDigest, evidence.Identity.BuildArtifact.Digest,
		evidence.Identity.PublicationArtifact.Digest, evidence.Identity.Provenance.ProvenanceDigest,
		evidence.SystemTests.FixturePublicationArtifact.Digest,
		evidence.SystemTests.Provenance.AttestationManifestDigest,
	} {
		if !digestPattern.MatchString(digest) {
			return fmt.Errorf("evidence contains non-immutable digest %q", digest)
		}
	}
	for _, hash := range []string{
		evidence.Gateway.BinarySHA256, evidence.Admin.ArchiveSHA256,
		evidence.Identity.PublicationRecordSHA256, evidence.SystemTests.FixtureBinarySHA256,
	} {
		if !shaPattern.MatchString(hash) {
			return fmt.Errorf("evidence contains malformed SHA-256 %q", hash)
		}
	}
	return nil
}

func equalStrings(left, right *string) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

func stringPointer(value string) *string {
	return &value
}
