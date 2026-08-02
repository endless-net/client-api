package releasecontract

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"reflect"
	"regexp"
	"strings"
)

const (
	ReleasedEnvelopeSchema = "https://endlessnet.ru/contracts/release/v1/released-envelope.schema.json"
	ResolutionSchema       = "https://endlessnet.ru/contracts/release/v1/resolution.schema.json"
)

var (
	digestPattern            = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
	commitPattern            = regexp.MustCompile(`^[0-9a-f]{40}$`)
	releasePattern           = regexp.MustCompile(`^[0-9A-Za-z._-]+$`)
	repositoryPattern        = regexp.MustCompile(`^endless-net/[0-9A-Za-z._-]+$`)
	artifactReferencePattern = regexp.MustCompile(`^\S+@sha256:[0-9a-f]{64}$`)
	githubBlobPattern        = regexp.MustCompile(`^https://github\.com/endless-net/[0-9A-Za-z._-]+/blob/[0-9a-f]{40}/[^?#]+$`)
	actionsRunPattern        = regexp.MustCompile(`^https://github\.com/endless-net/[0-9A-Za-z._-]+/actions/runs/[1-9][0-9]*$`)
	versionPattern           = regexp.MustCompile(`^v[0-9]+\.[0-9]+\.[0-9]+(?:-[0-9A-Za-z.-]+)?$`)
	sumPattern               = regexp.MustCompile(`^h1:[A-Za-z0-9+/=]+$`)
)

func Digest(raw []byte) string {
	digest := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(digest[:])
}

func DecodeStrict(raw []byte, value any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("document contains trailing JSON")
		}
		return fmt.Errorf("trailing JSON: %w", err)
	}
	return nil
}

func ValidateCandidate(candidate Candidate) error {
	if candidate.SchemaVersion != 1 || !releasePattern.MatchString(candidate.Release) || candidate.Status != "candidate" {
		return errors.New("candidate header is invalid")
	}
	if len(candidate.Components) == 0 {
		return errors.New("candidate has no components")
	}
	names := make(map[string]bool, len(candidate.Components))
	repositories := make(map[string]bool, len(candidate.Components))
	for index, component := range candidate.Components {
		if !releasePattern.MatchString(component.Name) || !repositoryPattern.MatchString(component.Repository) ||
			!commitPattern.MatchString(component.GitCommit) {
			return fmt.Errorf("component[%d] identity is invalid", index)
		}
		if names[component.Name] || repositories[component.Repository] {
			return fmt.Errorf("component[%d] duplicates a name or repository", index)
		}
		names[component.Name], repositories[component.Repository] = true, true
		if component.Image != nil && !artifactReferencePattern.MatchString(*component.Image) {
			return fmt.Errorf("component %q has mutable artifact reference %q", component.Name, *component.Image)
		}
	}
	modulePaths := make(map[string]bool, len(candidate.Modules))
	for index, module := range candidate.Modules {
		if module.Path == "" || modulePaths[module.Path] || !versionPattern.MatchString(module.Version) || !sumPattern.MatchString(module.Sum) {
			return fmt.Errorf("module[%d] is invalid or duplicated", index)
		}
		modulePaths[module.Path] = true
	}
	contractNames := make(map[string]bool, len(candidate.Contracts))
	for index, contract := range candidate.Contracts {
		if contract.Name == "" || contractNames[contract.Name] || contract.Version < 1 ||
			contract.ArchitectureRepository != "endless-net/architecture" || !commitPattern.MatchString(contract.ArchitectureCommit) {
			return fmt.Errorf("contract[%d] is invalid or duplicated", index)
		}
		contractNames[contract.Name] = true
	}
	return nil
}

func ValidateCandidateProvenance(candidate Candidate, candidateDigest string, provenance CandidateProvenance) error {
	if err := ValidateCandidate(candidate); err != nil {
		return err
	}
	if !digestPattern.MatchString(candidateDigest) || provenance.SchemaVersion != 1 ||
		provenance.Release != candidate.Release || provenance.CandidateDigest != candidateDigest {
		return errors.New("candidate provenance header does not bind the candidate")
	}
	if len(provenance.Components) != len(candidate.Components) || len(provenance.Modules) != len(candidate.Modules) {
		return errors.New("candidate provenance does not cover the complete resolved set")
	}
	components := make(map[string]ComponentProvenance, len(provenance.Components))
	for _, item := range provenance.Components {
		if _, exists := components[item.Name]; exists {
			return fmt.Errorf("candidate provenance duplicates component %q", item.Name)
		}
		components[item.Name] = item
	}
	for _, component := range candidate.Components {
		item, ok := components[component.Name]
		if !ok {
			return fmt.Errorf("candidate provenance is missing component %q", component.Name)
		}
		if component.Image == nil {
			return fmt.Errorf("component %q has no immutable artifact and cannot be promoted", component.Name)
		}
		digest := artifactDigest(*component.Image)
		if item.Repository != component.Repository || item.GitCommit != component.GitCommit ||
			item.Artifact.URI != *component.Image || item.Artifact.Digest != digest {
			return fmt.Errorf("candidate provenance drifts from component %q", component.Name)
		}
		if item.Build.Repository != component.Repository || item.Build.SourceCommit != component.GitCommit ||
			!actionsRunFor(component.Repository, item.Build.Run) || !digestPattern.MatchString(item.Build.AttestationDigest) {
			return fmt.Errorf("candidate provenance for component %q is incomplete", component.Name)
		}
	}
	modules := make(map[string]ModuleProvenance, len(provenance.Modules))
	for _, item := range provenance.Modules {
		if _, exists := modules[item.Path]; exists {
			return fmt.Errorf("candidate provenance duplicates module %q", item.Path)
		}
		modules[item.Path] = item
	}
	for _, module := range candidate.Modules {
		item, ok := modules[module.Path]
		if !ok || item.Version != module.Version || item.Sum != module.Sum ||
			!repositoryPattern.MatchString(item.Source.Repository) || !commitPattern.MatchString(item.Source.GitCommit) ||
			!actionsRunFor(item.Source.Repository, item.Source.Run) {
			return fmt.Errorf("candidate provenance drifts from or is missing module %q", module.Path)
		}
	}
	return nil
}

func ValidateSystemTestEvidence(candidateDigest string, evidence SystemTestEvidence) error {
	if evidence.SchemaVersion != 1 || evidence.CandidateDigest != candidateDigest || evidence.Result != "passed" ||
		evidence.Suite.Repository != "endless-net/system-tests" || !commitPattern.MatchString(evidence.Suite.GitCommit) ||
		!actionsRunFor(evidence.Suite.Repository, evidence.Suite.Run) {
		return errors.New("system-test evidence is incomplete or does not bind a passing run to the candidate")
	}
	return nil
}

func BuildReleasedEnvelope(candidate Candidate, candidateRaw, provenanceRaw, systemTestRaw []byte, provenance CandidateProvenance, evidence SystemTestEvidence, request PromotionRequest) (ReleasedEnvelope, error) {
	candidateDigest := Digest(candidateRaw)
	if request.SchemaVersion != 1 || request.Candidate.Digest != candidateDigest ||
		request.CandidateProvenance.Digest != Digest(provenanceRaw) || request.SystemTestEvidence.Digest != Digest(systemTestRaw) {
		return ReleasedEnvelope{}, errors.New("promotion request digest does not match an input record")
	}
	for name, reference := range map[string]DigestReference{
		"candidate":            request.Candidate,
		"candidate provenance": request.CandidateProvenance,
		"system-test evidence": request.SystemTestEvidence,
	} {
		if err := validateDigestReference(reference); err != nil {
			return ReleasedEnvelope{}, fmt.Errorf("%s: %w", name, err)
		}
	}
	if request.Promotion.Repository != "endless-net/client-api" || !actionsRunFor(request.Promotion.Repository, request.Promotion.Run) {
		return ReleasedEnvelope{}, errors.New("promotion provenance is incomplete")
	}
	if err := ValidateCandidateProvenance(candidate, candidateDigest, provenance); err != nil {
		return ReleasedEnvelope{}, err
	}
	if err := ValidateSystemTestEvidence(candidateDigest, evidence); err != nil {
		return ReleasedEnvelope{}, err
	}
	return ReleasedEnvelope{
		Schema:        ReleasedEnvelopeSchema,
		SchemaVersion: 1,
		Release:       candidate.Release,
		Status:        "released",
		Candidate:     request.Candidate,
		Resolved:      candidate.Resolved(),
		Provenance: ReleaseProvenance{
			CandidateProvenance: request.CandidateProvenance,
			SystemTests: SystemTestProvenance{
				Evidence:   request.SystemTestEvidence,
				Repository: evidence.Suite.Repository,
				GitCommit:  evidence.Suite.GitCommit,
				Run:        evidence.Suite.Run,
			},
			Promotion: request.Promotion,
		},
	}, nil
}

func ValidateReleasedEnvelope(envelope ReleasedEnvelope, candidate Candidate, candidateRaw, provenanceRaw, systemTestRaw []byte, provenance CandidateProvenance, evidence SystemTestEvidence) error {
	request := PromotionRequest{
		SchemaVersion:       1,
		Candidate:           envelope.Candidate,
		CandidateProvenance: envelope.Provenance.CandidateProvenance,
		SystemTestEvidence:  envelope.Provenance.SystemTests.Evidence,
		Promotion:           envelope.Provenance.Promotion,
	}
	expected, err := BuildReleasedEnvelope(candidate, candidateRaw, provenanceRaw, systemTestRaw, provenance, evidence, request)
	if err != nil {
		return err
	}
	if envelope.SchemaVersion != 1 || envelope.Status != "released" || envelope.Release != candidate.Release {
		return errors.New("released envelope header is invalid")
	}
	if !reflect.DeepEqual(envelope.Resolved, candidate.Resolved()) {
		return errors.New("released envelope resolved set drifts from the tested candidate")
	}
	if !reflect.DeepEqual(envelope.Candidate, expected.Candidate) || !reflect.DeepEqual(envelope.Provenance, expected.Provenance) {
		return errors.New("released envelope provenance does not bind the promotion inputs")
	}
	return nil
}

func ResolveCandidate(candidate Candidate, candidateRaw, provenanceRaw []byte, provenance CandidateProvenance, candidateReference, provenanceReference DigestReference) (Resolution, error) {
	candidateDigest := Digest(candidateRaw)
	if candidateReference.Digest != candidateDigest || provenanceReference.Digest != Digest(provenanceRaw) {
		return Resolution{}, errors.New("validation resolution reference digest does not match its record")
	}
	if err := validateDigestReference(candidateReference); err != nil {
		return Resolution{}, err
	}
	if err := validateDigestReference(provenanceReference); err != nil {
		return Resolution{}, err
	}
	if err := ValidateCandidateProvenance(candidate, candidateDigest, provenance); err != nil {
		return Resolution{}, err
	}
	return Resolution{
		Schema:           ResolutionSchema,
		SchemaVersion:    1,
		EnvironmentClass: "validation",
		Source:           ResolutionSource{Kind: "candidate", Record: candidateReference},
		Release:          candidate.Release,
		CandidateDigest:  candidateDigest,
		Resolved:         candidate.Resolved(),
		Provenance:       ResolutionProvenance{CandidateProvenance: provenanceReference},
	}, nil
}

func ResolveProduction(envelope ReleasedEnvelope, envelopeRaw []byte, envelopeReference DigestReference) (Resolution, error) {
	if envelope.SchemaVersion != 1 || envelope.Status != "released" || !releasePattern.MatchString(envelope.Release) {
		return Resolution{}, errors.New("production accepts only a released envelope")
	}
	if envelopeReference.Digest != Digest(envelopeRaw) {
		return Resolution{}, errors.New("released envelope reference digest does not match its record")
	}
	if err := validateDigestReference(envelopeReference); err != nil {
		return Resolution{}, err
	}
	if err := validateDigestReference(envelope.Candidate); err != nil {
		return Resolution{}, err
	}
	if err := validateDigestReference(envelope.Provenance.CandidateProvenance); err != nil {
		return Resolution{}, err
	}
	if err := validateDigestReference(envelope.Provenance.SystemTests.Evidence); err != nil {
		return Resolution{}, err
	}
	if envelope.Provenance.SystemTests.Repository != "endless-net/system-tests" ||
		!commitPattern.MatchString(envelope.Provenance.SystemTests.GitCommit) ||
		!actionsRunFor(envelope.Provenance.SystemTests.Repository, envelope.Provenance.SystemTests.Run) ||
		envelope.Provenance.Promotion.Repository != "endless-net/client-api" ||
		!actionsRunFor(envelope.Provenance.Promotion.Repository, envelope.Provenance.Promotion.Run) {
		return Resolution{}, errors.New("released envelope provenance is incomplete")
	}
	if err := ValidateCandidate(Candidate{
		SchemaVersion: 1,
		Release:       envelope.Release,
		Status:        "candidate",
		Components:    envelope.Resolved.Components,
		Modules:       envelope.Resolved.Modules,
		Contracts:     envelope.Resolved.Contracts,
	}); err != nil {
		return Resolution{}, fmt.Errorf("released resolved set: %w", err)
	}
	for _, component := range envelope.Resolved.Components {
		if component.Image == nil || !artifactReferencePattern.MatchString(*component.Image) {
			return Resolution{}, fmt.Errorf("released component %q lacks an immutable artifact", component.Name)
		}
	}
	systemTests, promotion := envelope.Provenance.SystemTests, envelope.Provenance.Promotion
	return Resolution{
		Schema:           ResolutionSchema,
		SchemaVersion:    1,
		EnvironmentClass: "production",
		Source: ResolutionSource{
			Kind:   "released",
			Record: envelopeReference,
		},
		Release:         envelope.Release,
		CandidateDigest: envelope.Candidate.Digest,
		Resolved:        envelope.Resolved,
		Provenance: ResolutionProvenance{
			CandidateProvenance: envelope.Provenance.CandidateProvenance,
			SystemTests:         &systemTests,
			Promotion:           &promotion,
		},
	}, nil
}

func WriteNewJSON(path string, value any) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	encodeErr := encoder.Encode(value)
	closeErr := file.Close()
	if encodeErr != nil {
		return encodeErr
	}
	return closeErr
}

func ValidateImmutableChanges(changes string) error {
	for _, line := range strings.Split(strings.TrimSpace(changes), "\n") {
		fields := strings.Split(strings.TrimSuffix(line, "\r"), "\t")
		if len(fields) < 2 {
			continue
		}
		status := fields[0]
		for _, path := range fields[1:] {
			path = strings.ReplaceAll(path, "\\", "/")
			if frozenLegacyReleasePath(path) {
				return fmt.Errorf("frozen legacy server-release source %q has forbidden git status %q", path, status)
			}
		}
	}
	return nil
}

func frozenLegacyReleasePath(path string) bool {
	if path == "release/manifest.schema.json" || path == "release/evidence.schema.json" {
		return true
	}
	for _, prefix := range []string{
		"release/candidates/",
		"release/candidate-provenance/",
		"release/system-test-evidence/",
		"release/releases/",
		"release/evidence/",
		"release/schemas/",
		"release/fixtures/",
	} {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}

func validateDigestReference(reference DigestReference) error {
	if !digestPattern.MatchString(reference.Digest) {
		return fmt.Errorf("invalid digest %q", reference.Digest)
	}
	if !githubBlobPattern.MatchString(reference.URI) && !artifactReferencePattern.MatchString(reference.URI) {
		return fmt.Errorf("mutable or unsupported URI %q", reference.URI)
	}
	return nil
}

func artifactDigest(reference string) string {
	position := strings.LastIndex(reference, "@sha256:")
	if position < 0 {
		return ""
	}
	return reference[position+1:]
}

func actionsRunFor(repository, run string) bool {
	return actionsRunPattern.MatchString(run) && strings.HasPrefix(run, "https://github.com/"+repository+"/actions/runs/")
}
