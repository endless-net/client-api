package systemtests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

const (
	recoveryContractName       = "client-signing-identity-recovery"
	recoveryArchitectureCommit = "6cf37091846920e238bef631ef8951d395c084a1"
	recoveryClientAPIModule    = "github.com/endless-net/client-api/clientapi/v2"
)

type releaseComponent struct {
	Name       string  `json:"name"`
	Repository string  `json:"repository"`
	GitCommit  string  `json:"git_commit"`
	Image      *string `json:"image"`
}

type releaseModule struct {
	Path    string `json:"path"`
	Version string `json:"version"`
	Sum     string `json:"sum"`
}

type releaseContract struct {
	Name                   string `json:"name"`
	Version                int    `json:"version"`
	ArchitectureRepository string `json:"architecture_repository"`
	ArchitectureCommit     string `json:"architecture_commit"`
}

type releaseManifest struct {
	Schema        string             `json:"$schema,omitempty"`
	SchemaVersion int                `json:"schema_version"`
	Release       string             `json:"release"`
	Status        string             `json:"status"`
	Components    []releaseComponent `json:"components"`
	Modules       []releaseModule    `json:"modules"`
	Contracts     []releaseContract  `json:"contracts,omitempty"`
}

func TestCandidateManifestsAreCompleteAndImmutable(t *testing.T) {
	files, err := filepath.Glob("../release/candidates/*.json")
	if err != nil || len(files) == 0 {
		t.Fatalf("candidate manifests: files=%v err=%v", files, err)
	}
	required := map[string]bool{
		"gateway": false, "admin": false, "infrastructure": false,
		"identity": false, "coordinator": false, "billing": false,
		"management": false, "signing": false, "mcp": false, "relay": false,
		"stun": false, "client": false, "observability": false,
	}
	commitPattern := regexp.MustCompile(`^[0-9a-f]{40}$`)
	digestPattern := regexp.MustCompile(`^.+@sha256:[0-9a-f]{64}$`)
	versionPattern := regexp.MustCompile(`^v[0-9]+\.[0-9]+\.[0-9]+(?:-[0-9A-Za-z.-]+)?$`)
	sumPattern := regexp.MustCompile(`^h1:[A-Za-z0-9+/=]+$`)
	for _, file := range files {
		raw, readErr := os.ReadFile(file)
		if readErr != nil {
			t.Fatal(readErr)
		}
		var manifest releaseManifest
		decoder := json.NewDecoder(bytes.NewReader(raw))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&manifest); err != nil {
			t.Fatalf("%s: %v", file, err)
		}
		if manifest.SchemaVersion != 1 || manifest.Release == "" ||
			manifest.Status != "candidate" && manifest.Status != "released" {
			t.Fatalf("%s has invalid header", file)
		}
		names, repositories := map[string]bool{}, map[string]bool{}
		for _, component := range manifest.Components {
			if component.Name == "" || names[component.Name] || repositories[component.Repository] ||
				!strings.HasPrefix(component.Repository, "endless-net/") || !commitPattern.MatchString(component.GitCommit) {
				t.Fatalf("%s has invalid or duplicate component %#v", file, component)
			}
			names[component.Name], repositories[component.Repository] = true, true
			if _, tracked := required[component.Name]; tracked {
				required[component.Name] = true
			}
			if manifest.Status == "released" && (component.Image == nil || !digestPattern.MatchString(*component.Image)) {
				t.Fatalf("%s released component %s lacks immutable image", file, component.Name)
			}
			if component.Image != nil && !digestPattern.MatchString(*component.Image) {
				t.Fatalf("%s component %s has mutable image", file, component.Name)
			}
		}
		modulePaths := map[string]bool{}
		for _, module := range manifest.Modules {
			if modulePaths[module.Path] || !versionPattern.MatchString(module.Version) ||
				!sumPattern.MatchString(module.Sum) {
				t.Fatalf("%s has invalid or duplicate module %#v", file, module)
			}
			modulePaths[module.Path] = true
		}
		contractNames := map[string]bool{}
		for _, contract := range manifest.Contracts {
			if contract.Name == "" || contractNames[contract.Name] || contract.Version < 1 ||
				contract.ArchitectureRepository != "endless-net/architecture" ||
				!commitPattern.MatchString(contract.ArchitectureCommit) {
				t.Fatalf("%s has invalid or duplicate contract %#v", file, contract)
			}
			contractNames[contract.Name] = true
		}
		if err := validateRecoveryContractPins(manifest); err != nil {
			t.Fatalf("%s: %v", file, err)
		}
		if err := validateGatewayBrowserLoginPins(manifest); err != nil {
			t.Fatalf("%s: %v", file, err)
		}
	}
	for component, present := range required {
		if !present {
			t.Errorf("candidate manifests do not pin %s", component)
		}
	}
}

func validateRecoveryContractPins(manifest releaseManifest) error {
	claimed := false
	for _, contract := range manifest.Contracts {
		if contract.Name != recoveryContractName {
			continue
		}
		claimed = true
		if contract.Version != 1 || contract.ArchitectureRepository != "endless-net/architecture" ||
			contract.ArchitectureCommit != recoveryArchitectureCommit {
			return fmt.Errorf("%s must pin version 1 at architecture commit %s", recoveryContractName, recoveryArchitectureCommit)
		}
	}
	if !claimed {
		return nil
	}
	repositories := make(map[string]bool, len(manifest.Components))
	for _, component := range manifest.Components {
		repositories[component.Repository] = true
	}
	for _, repository := range []string{
		"endless-net/client-api",
		"endless-net/coordinator",
		"endless-net/gateway",
		"endless-net/client",
		"endless-net/client-ui",
		"endless-net/system-tests",
	} {
		if !repositories[repository] {
			return fmt.Errorf("%s requires an exact %s component commit", recoveryContractName, repository)
		}
	}
	for _, module := range manifest.Modules {
		if module.Path == recoveryClientAPIModule && strings.HasPrefix(module.Version, "v2.") && strings.HasPrefix(module.Sum, "h1:") {
			return nil
		}
	}
	return fmt.Errorf("%s requires a checksummed v2 pin of %s", recoveryContractName, recoveryClientAPIModule)
}
