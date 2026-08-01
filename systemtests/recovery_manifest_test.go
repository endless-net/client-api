package systemtests

import (
	"strings"
	"testing"
)

func TestRecoveryContractManifestRequirements(t *testing.T) {
	valid := recoveryManifestFixture()
	if err := validateRecoveryContractPins(valid); err != nil {
		t.Fatal(err)
	}

	tests := map[string]func(*releaseManifest){
		"architecture commit": func(manifest *releaseManifest) {
			manifest.Contracts[0].ArchitectureCommit = strings.Repeat("f", 40)
		},
		"client api v2 module": func(manifest *releaseManifest) {
			manifest.Modules[0].Path = "github.com/endless-net/client-api/clientapi"
		},
		"v2 module version": func(manifest *releaseManifest) {
			manifest.Modules[0].Version = "v1.3.0"
		},
		"coordinator commit": func(manifest *releaseManifest) {
			manifest.Components = manifest.Components[:2]
		},
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			manifest := recoveryManifestFixture()
			mutate(&manifest)
			if err := validateRecoveryContractPins(manifest); err == nil {
				t.Fatal("invalid recovery manifest was accepted")
			}
		})
	}
}

func TestManifestWithoutRecoveryContractDoesNotRequireV2Pins(t *testing.T) {
	if err := validateRecoveryContractPins(releaseManifest{}); err != nil {
		t.Fatal(err)
	}
}

func recoveryManifestFixture() releaseManifest {
	commit := strings.Repeat("a", 40)
	components := make([]releaseComponent, 0, 6)
	for index, repository := range []string{
		"endless-net/client-api",
		"endless-net/coordinator",
		"endless-net/gateway",
		"endless-net/client",
		"endless-net/client-ui",
		"endless-net/system-tests",
	} {
		components = append(components, releaseComponent{
			Name:       repository[strings.LastIndex(repository, "/")+1:] + string(rune('0'+index)),
			Repository: repository,
			GitCommit:  commit,
		})
	}
	return releaseManifest{
		SchemaVersion: 1,
		Release:       "recovery-rc1",
		Status:        "candidate",
		Components:    components,
		Modules: []releaseModule{{
			Path: recoveryClientAPIModule, Version: "v2.0.0-rc.1", Sum: "h1:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=",
		}},
		Contracts: []releaseContract{{
			Name: recoveryContractName, Version: 1,
			ArchitectureRepository: "endless-net/architecture",
			ArchitectureCommit:     recoveryArchitectureCommit,
		}},
	}
}
