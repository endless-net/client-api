package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"

	"github.com/endless-net/client-api/systemtests/releasecontract"
)

func main() {
	if len(os.Args) < 2 {
		fatalf("usage: releasecontract <promote|resolve-candidate|resolve-release|verify-history> [flags]")
	}
	switch os.Args[1] {
	case "promote":
		promote(os.Args[2:])
	case "resolve-candidate":
		resolveCandidate(os.Args[2:])
	case "resolve-release":
		resolveRelease(os.Args[2:])
	case "verify-history":
		verifyHistory(os.Args[2:])
	default:
		fatalf("unknown command %q", os.Args[1])
	}
}

func resolveCandidate(arguments []string) {
	flags := flag.NewFlagSet("resolve-candidate", flag.ExitOnError)
	candidatePath := flags.String("candidate", "", "candidate manifest path")
	provenancePath := flags.String("candidate-provenance", "", "candidate provenance path")
	candidateURI := flags.String("candidate-uri", "", "immutable candidate URI")
	provenanceURI := flags.String("candidate-provenance-uri", "", "immutable candidate provenance URI")
	_ = flags.Parse(arguments)
	if *candidatePath == "" || *provenancePath == "" || *candidateURI == "" || *provenanceURI == "" {
		flags.Usage()
		os.Exit(2)
	}
	candidateRaw, candidate := read[releasecontract.Candidate](*candidatePath)
	provenanceRaw, provenance := read[releasecontract.CandidateProvenance](*provenancePath)
	resolution, err := releasecontract.ResolveCandidate(
		candidate,
		candidateRaw,
		provenanceRaw,
		provenance,
		releasecontract.DigestReference{URI: *candidateURI, Digest: releasecontract.Digest(candidateRaw)},
		releasecontract.DigestReference{URI: *provenanceURI, Digest: releasecontract.Digest(provenanceRaw)},
	)
	if err != nil {
		fatalf("resolve candidate: %v", err)
	}
	writeJSON(resolution)
}

func resolveRelease(arguments []string) {
	flags := flag.NewFlagSet("resolve-release", flag.ExitOnError)
	envelopePath := flags.String("envelope", "", "released envelope path")
	candidatePath := flags.String("candidate", "", "referenced candidate manifest path")
	provenancePath := flags.String("candidate-provenance", "", "referenced candidate provenance path")
	systemTestPath := flags.String("system-test-evidence", "", "referenced passing system-test evidence path")
	envelopeURI := flags.String("envelope-uri", "", "immutable released envelope URI")
	_ = flags.Parse(arguments)
	if *envelopePath == "" || *candidatePath == "" || *provenancePath == "" || *systemTestPath == "" || *envelopeURI == "" {
		flags.Usage()
		os.Exit(2)
	}
	envelopeRaw, envelope := read[releasecontract.ReleasedEnvelope](*envelopePath)
	candidateRaw, candidate := read[releasecontract.Candidate](*candidatePath)
	provenanceRaw, provenance := read[releasecontract.CandidateProvenance](*provenancePath)
	systemTestRaw, evidence := read[releasecontract.SystemTestEvidence](*systemTestPath)
	if err := releasecontract.ValidateReleasedEnvelope(
		envelope,
		candidate,
		candidateRaw,
		provenanceRaw,
		systemTestRaw,
		provenance,
		evidence,
	); err != nil {
		fatalf("verify released envelope: %v", err)
	}
	resolution, err := releasecontract.ResolveProduction(
		envelope,
		envelopeRaw,
		releasecontract.DigestReference{URI: *envelopeURI, Digest: releasecontract.Digest(envelopeRaw)},
	)
	if err != nil {
		fatalf("resolve release: %v", err)
	}
	writeJSON(resolution)
}

func promote(arguments []string) {
	flags := flag.NewFlagSet("promote", flag.ExitOnError)
	candidatePath := flags.String("candidate", "", "candidate manifest path")
	provenancePath := flags.String("candidate-provenance", "", "candidate provenance path")
	systemTestPath := flags.String("system-test-evidence", "", "passing system-test evidence path")
	requestPath := flags.String("request", "", "promotion request path")
	outputPath := flags.String("output", "", "new released envelope path")
	_ = flags.Parse(arguments)
	if *candidatePath == "" || *provenancePath == "" || *systemTestPath == "" || *requestPath == "" || *outputPath == "" {
		flags.Usage()
		os.Exit(2)
	}
	candidateRaw, candidate := read[releasecontract.Candidate](*candidatePath)
	provenanceRaw, provenance := read[releasecontract.CandidateProvenance](*provenancePath)
	systemTestRaw, evidence := read[releasecontract.SystemTestEvidence](*systemTestPath)
	_, request := read[releasecontract.PromotionRequest](*requestPath)
	envelope, err := releasecontract.BuildReleasedEnvelope(candidate, candidateRaw, provenanceRaw, systemTestRaw, provenance, evidence, request)
	if err != nil {
		fatalf("promote: %v", err)
	}
	if err := releasecontract.WriteNewJSON(*outputPath, envelope); err != nil {
		fatalf("write immutable released envelope: %v", err)
	}
}

func verifyHistory(arguments []string) {
	flags := flag.NewFlagSet("verify-history", flag.ExitOnError)
	base := flags.String("base", "", "base git revision")
	head := flags.String("head", "HEAD", "head git revision")
	_ = flags.Parse(arguments)
	if *base == "" {
		flags.Usage()
		os.Exit(2)
	}
	command := exec.Command("git", "log", "--format=", "--name-status", "--find-renames", "--reverse", *base+".."+*head, "--", ":(top)release")
	output, err := command.Output()
	if err != nil {
		fatalf("git history: %v", err)
	}
	if err := releasecontract.ValidateImmutableChanges(string(output)); err != nil {
		fatalf("verify history: %v", err)
	}
}

func read[T any](path string) ([]byte, T) {
	var value T
	raw, err := os.ReadFile(path)
	if err != nil {
		fatalf("read %s: %v", path, err)
	}
	if err := releasecontract.DecodeStrict(raw, &value); err != nil {
		fatalf("decode %s: %v", path, err)
	}
	return raw, value
}

func writeJSON(value any) {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(value); err != nil {
		fatalf("encode output: %v", err)
	}
}

func fatalf(format string, arguments ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", arguments...)
	os.Exit(1)
}
