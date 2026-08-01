package systemtests

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

type releaseManifest struct {
	SchemaVersion int    `json:"schema_version"`
	Release       string `json:"release"`
	Status        string `json:"status"`
	Components    []struct {
		Name       string  `json:"name"`
		Repository string  `json:"repository"`
		GitCommit  string  `json:"git_commit"`
		Image      *string `json:"image"`
	} `json:"components"`
	Modules []struct {
		Path    string `json:"path"`
		Version string `json:"version"`
		Sum     string `json:"sum"`
	} `json:"modules"`
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
		if err := json.Unmarshal(raw, &manifest); err != nil {
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
	}
	for component, present := range required {
		if !present {
			t.Errorf("candidate manifests do not pin %s", component)
		}
	}
}
