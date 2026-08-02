// Package releasecontract implements the semantic checks which JSON Schema
// cannot express for the historical EndlessNet D-025 release contract.
//
// Deprecated: server release-control ownership moved to the private
// github.com/endless-net/releases repository. This package is retained as a
// frozen migration source until Releases, System Tests, and Infrastructure
// provide copy and consumer-cutover evidence.
package releasecontract

type DigestReference struct {
	URI    string `json:"uri"`
	Digest string `json:"digest"`
}

type Component struct {
	Name       string  `json:"name"`
	Repository string  `json:"repository"`
	GitCommit  string  `json:"git_commit"`
	Image      *string `json:"image"`
}

type Module struct {
	Path    string `json:"path"`
	Version string `json:"version"`
	Sum     string `json:"sum"`
}

type Contract struct {
	Name                   string `json:"name"`
	Version                int    `json:"version"`
	ArchitectureRepository string `json:"architecture_repository"`
	ArchitectureCommit     string `json:"architecture_commit"`
}

type ResolvedSet struct {
	Components []Component `json:"components"`
	Modules    []Module    `json:"modules"`
	Contracts  []Contract  `json:"contracts,omitempty"`
}

type Candidate struct {
	Schema        string      `json:"$schema,omitempty"`
	SchemaVersion int         `json:"schema_version"`
	Release       string      `json:"release"`
	Status        string      `json:"status"`
	Components    []Component `json:"components"`
	Modules       []Module    `json:"modules"`
	Contracts     []Contract  `json:"contracts,omitempty"`
}

func (candidate Candidate) Resolved() ResolvedSet {
	return ResolvedSet{
		Components: candidate.Components,
		Modules:    candidate.Modules,
		Contracts:  candidate.Contracts,
	}
}

type BuildProvenance struct {
	Repository        string `json:"repository"`
	SourceCommit      string `json:"source_commit"`
	Run               string `json:"run"`
	AttestationDigest string `json:"attestation_digest"`
}

type ComponentProvenance struct {
	Name       string          `json:"name"`
	Repository string          `json:"repository"`
	GitCommit  string          `json:"git_commit"`
	Artifact   DigestReference `json:"artifact"`
	Build      BuildProvenance `json:"build"`
}

type ModuleSource struct {
	Repository string `json:"repository"`
	GitCommit  string `json:"git_commit"`
	Run        string `json:"run"`
}

type ModuleProvenance struct {
	Path    string       `json:"path"`
	Version string       `json:"version"`
	Sum     string       `json:"sum"`
	Source  ModuleSource `json:"source"`
}

type CandidateProvenance struct {
	Schema          string                `json:"$schema"`
	SchemaVersion   int                   `json:"schema_version"`
	Release         string                `json:"release"`
	CandidateDigest string                `json:"candidate_digest"`
	Components      []ComponentProvenance `json:"components"`
	Modules         []ModuleProvenance    `json:"modules"`
}

type TestSuiteProvenance struct {
	Repository string `json:"repository"`
	GitCommit  string `json:"git_commit"`
	Run        string `json:"run"`
}

type SystemTestEvidence struct {
	Schema          string              `json:"$schema"`
	SchemaVersion   int                 `json:"schema_version"`
	CandidateDigest string              `json:"candidate_digest"`
	Result          string              `json:"result"`
	Suite           TestSuiteProvenance `json:"suite"`
}

type PromotionProvenance struct {
	Repository string `json:"repository"`
	Run        string `json:"run"`
}

type PromotionRequest struct {
	Schema              string              `json:"$schema"`
	SchemaVersion       int                 `json:"schema_version"`
	Candidate           DigestReference     `json:"candidate"`
	CandidateProvenance DigestReference     `json:"candidate_provenance"`
	SystemTestEvidence  DigestReference     `json:"system_test_evidence"`
	Promotion           PromotionProvenance `json:"promotion"`
}

type SystemTestProvenance struct {
	Evidence   DigestReference `json:"evidence"`
	Repository string          `json:"repository"`
	GitCommit  string          `json:"git_commit"`
	Run        string          `json:"run"`
}

type ReleaseProvenance struct {
	CandidateProvenance DigestReference      `json:"candidate_provenance"`
	SystemTests         SystemTestProvenance `json:"system_tests"`
	Promotion           PromotionProvenance  `json:"promotion"`
}

type ReleasedEnvelope struct {
	Schema        string            `json:"$schema"`
	SchemaVersion int               `json:"schema_version"`
	Release       string            `json:"release"`
	Status        string            `json:"status"`
	Candidate     DigestReference   `json:"candidate"`
	Resolved      ResolvedSet       `json:"resolved"`
	Provenance    ReleaseProvenance `json:"provenance"`
}

type ResolutionSource struct {
	Kind   string          `json:"kind"`
	Record DigestReference `json:"record"`
}

type ResolutionProvenance struct {
	CandidateProvenance DigestReference       `json:"candidate_provenance"`
	SystemTests         *SystemTestProvenance `json:"system_tests,omitempty"`
	Promotion           *PromotionProvenance  `json:"promotion,omitempty"`
}

type Resolution struct {
	Schema           string               `json:"$schema"`
	SchemaVersion    int                  `json:"schema_version"`
	EnvironmentClass string               `json:"environment_class"`
	Source           ResolutionSource     `json:"source"`
	Release          string               `json:"release"`
	CandidateDigest  string               `json:"candidate_digest"`
	Resolved         ResolvedSet          `json:"resolved"`
	Provenance       ResolutionProvenance `json:"provenance"`
}
