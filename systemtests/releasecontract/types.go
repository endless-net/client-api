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
	ApprovedBy string `json:"approved_by,omitempty"`
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

// ComponentCandidate is the D-025 component-scoped validation input.  It is
// deliberately not a server snapshot: one producer publishes one candidate.
type ComponentCandidate struct {
	Schema             string                       `json:"$schema"`
	SchemaVersion      int                          `json:"schema_version"`
	Kind               string                       `json:"kind"`
	Component          string                       `json:"component"`
	Repository         string                       `json:"repository"`
	GitCommit          string                       `json:"git_commit"`
	Artifact           string                       `json:"artifact"`
	Modules            []Module                     `json:"modules,omitempty"`
	Contracts          []Contract                   `json:"contracts,omitempty"`
	CompatibilityGates []CompatibilityGate          `json:"compatibility_gates"`
	Provenance         ComponentCandidateProvenance `json:"provenance"`
}

type CompatibilityGate struct {
	Provider string `json:"provider"`
	Consumer string `json:"consumer"`
	Contract string `json:"contract"`
	Version  int    `json:"version"`
}

type ComponentCandidateProvenance struct {
	ProducerRelease   DigestReference `json:"producer_release"`
	Build             BuildProvenance `json:"build"`
	AttestationDigest string          `json:"attestation_digest"`
}

// ReleasedComponent is immutable proof that one candidate passed its affected
// consumer/provider gates. It never implies a release of unrelated services.
type ReleasedComponent struct {
	Schema                string                  `json:"$schema"`
	SchemaVersion         int                     `json:"schema_version"`
	Kind                  string                  `json:"kind"`
	Component             string                  `json:"component"`
	Repository            string                  `json:"repository"`
	GitCommit             string                  `json:"git_commit"`
	Artifact              string                  `json:"artifact"`
	Candidate             DigestReference         `json:"candidate"`
	CandidateProvenance   DigestReference         `json:"candidate_provenance"`
	CompatibilityEvidence []CompatibilityEvidence `json:"compatibility_evidence"`
	Promotion             PromotionProvenance     `json:"promotion"`
}

type CompatibilityEvidence struct {
	Provider string          `json:"provider"`
	Consumer string          `json:"consumer"`
	Contract string          `json:"contract"`
	Version  int             `json:"version"`
	Evidence DigestReference `json:"evidence"`
}

// InfrastructureSignal is the sole producer-to-deployer notification. It
// carries no host, command, secret, or arbitrary artifact override.
type InfrastructureSignal struct {
	Schema           string          `json:"$schema"`
	SchemaVersion    int             `json:"schema_version"`
	Kind             string          `json:"kind"`
	Environment      string          `json:"environment"`
	Component        string          `json:"component"`
	ReleasedRecord   DigestReference `json:"released_record"`
	ManifestCommit   string          `json:"manifest_commit"`
	EventID          string          `json:"event_id"`
	ConfigGeneration string          `json:"config_generation"`
}
