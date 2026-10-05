package kconfig

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"sync"
)

const DefaultLLVMCapabilityProfile = "llvm-22"
const CompactGeneratorProtocol = "compact-v9-llvm-capabilities"

// Version retains the entire compatibility floor, including patch-level fixes.
type Version struct{ Major, Minor, Patch int }

func ParseVersion(value string) (Version, error) {
	parts := strings.Split(value, ".")
	var v Version
	if len(parts) != 3 {
		return v, fmt.Errorf("invalid LLVM version %q", value)
	}
	for i, field := range []*int{&v.Major, &v.Minor, &v.Patch} {
		n, err := strconv.Atoi(parts[i])
		if err != nil || n < 0 || (i > 0 && n > 99) || strconv.Itoa(n) != parts[i] {
			return Version{}, fmt.Errorf("invalid LLVM version %q", value)
		}
		*field = n
	}
	if v.Major < 1 || v.Major > 999 {
		return Version{}, fmt.Errorf("invalid LLVM version %q", value)
	}
	return v, nil
}

func (v Version) Encoded() int                 { return v.Major*10000 + v.Minor*100 + v.Patch }
func (v Version) String() string               { return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch) }
func (v Version) MarshalJSON() ([]byte, error) { return json.Marshal(v.String()) }
func (v *Version) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	parsed, err := ParseVersion(s)
	if err == nil {
		*v = parsed
	}
	return err
}

// CompilerCapabilities describes graph-generation policy, independently of the
// C++ toolchain eventually selected by Bazel. Source is already shell-decoded.
type CompilerCapabilities interface {
	Identity() string
	MinimumClangVersion() Version
	MinimumLLDVersion() Version
	SupportsOption(context.Context, string, []string, []string) (bool, error)
	SupportsSource(context.Context, string, string, []string) (bool, error)
}

type LLVMCapabilityProfile struct {
	Name          string  `json:"profile"`
	MinimumClang  Version `json:"minimum_clang"`
	MinimumLLD    Version `json:"minimum_lld"`
	ModelRevision string  `json:"model_revision"`
}

//go:embed llvm_profiles.json llvm_capabilities/*.json
var llvmCapabilityFiles embed.FS

var llvmProfiles = sync.OnceValues(func() (map[string]LLVMCapabilityProfile, error) {
	data, err := llvmCapabilityFiles.ReadFile("llvm_profiles.json")
	if err != nil {
		return nil, err
	}
	profiles := map[string]LLVMCapabilityProfile{}
	if err := json.Unmarshal(data, &profiles); err != nil {
		return nil, err
	}
	for name, profile := range profiles {
		profile.Name = name
		if name != fmt.Sprintf("llvm-%d", profile.MinimumClang.Major) || profile.MinimumLLD.Major != profile.MinimumClang.Major || !strings.HasPrefix(profile.ModelRevision, "capabilities-v") {
			return nil, fmt.Errorf("invalid LLVM capability profile %q", name)
		}
		profiles[name] = profile
	}
	return profiles, nil
})

func LLVMCapabilityProfileByName(name string) (LLVMCapabilityProfile, error) {
	profiles, err := llvmProfiles()
	if err != nil {
		return LLVMCapabilityProfile{}, err
	}
	profile, ok := profiles[name]
	if !ok {
		return LLVMCapabilityProfile{}, fmt.Errorf("unknown LLVM capability profile %q; expected llvm-19 through llvm-23", name)
	}
	return profile, nil
}

func (p LLVMCapabilityProfile) Identity(architecture string) string {
	return p.Name + "/" + p.ModelRevision + "/" + architecture
}
func (p LLVMCapabilityProfile) CompilerVersionText() string {
	return "clang version " + p.MinimumClang.String() + ", LLD " + p.MinimumLLD.String()
}

// CompilerCheckSource is deliberately freestanding and needs no kernel headers.
func (p LLVMCapabilityProfile) CompilerCheckSource() string {
	return fmt.Sprintf(`#ifndef __clang__
#error "linux.bzl requires Clang"
#else
#define LINUX_BZL_CLANG_VERSION (__clang_major__ * 10000 + __clang_minor__ * 100 + __clang_patchlevel__)
#if LINUX_BZL_CLANG_VERSION < %d
#error "linux.bzl %s profile requires Clang %s or newer"
#endif
#endif
typedef int linux_bzl_compiler_capability_check;
`, p.MinimumClang.Encoded(), p.Name, p.MinimumClang.String())
}

// CapabilityProbe is the reproducible, ordered input to a modeled probe. Path
// inputs which cannot affect these header-free tests are removed by the same
// normalization in the measured and static implementations.
type CapabilityProbe struct {
	Architecture string   `json:"architecture"`
	Kind         string   `json:"kind"`
	Candidate    []string `json:"candidate,omitempty"`
	Language     string   `json:"language,omitempty"`
	Source       string   `json:"source,omitempty"`
	Context      []string `json:"context,omitempty"`
}

type CapabilityRecord struct {
	CapabilityProbe
	Supported bool     `json:"supported"`
	Locations []string `json:"locations,omitempty"`
	Reason    string   `json:"reason,omitempty"`
}

type CapabilityDatabase struct {
	Profile           string             `json:"profile"`
	ReferenceClang    Version            `json:"reference_clang"`
	ReferenceLLD      Version            `json:"reference_lld"`
	ReferenceIdentity string             `json:"reference_identity"`
	Probes            []CapabilityRecord `json:"probes"`
}

func (p CapabilityProbe) Key() string {
	data, _ := json.Marshal(p)
	return string(data)
}

func normalizeCapabilityProbe(p CapabilityProbe) (CapabilityProbe, error) {
	kind := p.Kind
	if kind == "powerpc_script" {
		if len(p.Candidate) != 2 || len(p.Context) != 0 || (p.Candidate[0] != "gcc-check-mprofile-kernel.sh" && p.Candidate[0] != "gcc-check-fpatchable-function-entry.sh") || (p.Candidate[1] != "-mlittle-endian" && p.Candidate[1] != "-mbig-endian") {
			return p, fmt.Errorf("unknown PowerPC script probe")
		}
		return p, nil
	}
	if kind == "source" {
		kind = "cc_option"
		if p.Language == "assembler-with-cpp" {
			kind = "as_option"
		}
	}
	if kind != "cc_option" && kind != "as_option" && kind != "ld_option" {
		return p, fmt.Errorf("unknown probe kind %q", p.Kind)
	}
	if p.Kind == "source" && p.Language != "c" && p.Language != "assembler-with-cpp" {
		return p, fmt.Errorf("unknown source language %q", p.Language)
	}
	if p.Kind != "source" {
		if err := validateProbeCandidate(kind, p.Candidate); err != nil {
			return p, err
		}
	}
	flags, err := sanitizeProbeContext(kind, p.Context)
	if err != nil {
		return p, err
	}
	// Werror and integrated-as are intrinsic to the supported compiler contract.
	p.Context = nil
	for _, flag := range flags {
		if flag != "-Werror" && flag != "-fintegrated-as" {
			if linuxLLVMKbuildSupportsMacroPrefixMap(flag) {
				flag = "-fmacro-prefix-map=/__linux_bzl_source__/="
			}
			p.Context = append(p.Context, flag)
		}
	}
	p.Candidate = slices.Clone(p.Candidate)
	for i, flag := range p.Candidate {
		if linuxLLVMKbuildSupportsMacroPrefixMap(flag) {
			p.Candidate[i] = "-fmacro-prefix-map=/__linux_bzl_source__/="
		}
	}
	return p, nil
}

type staticLLVMCapabilities struct {
	profile      LLVMCapabilityProfile
	architecture string
	probes       map[string]CapabilityRecord
}

var capabilityDatabases sync.Map

func StaticLLVMCapabilities(name, architecture string) (CompilerCapabilities, error) {
	profile, err := LLVMCapabilityProfileByName(name)
	if err != nil {
		return nil, err
	}
	architecture, err = normalizeLinuxProbeArchitecture(architecture)
	if err != nil {
		return nil, err
	}
	value, ok := capabilityDatabases.Load(name)
	if !ok {
		data, err := llvmCapabilityFiles.ReadFile("llvm_capabilities/" + name + ".json")
		if err != nil {
			return nil, err
		}
		var db CapabilityDatabase
		if err := json.Unmarshal(data, &db); err != nil {
			return nil, err
		}
		index, err := capabilityIndex(db, profile)
		if err != nil {
			return nil, err
		}
		value, _ = capabilityDatabases.LoadOrStore(name, index)
	}
	return &staticLLVMCapabilities{profile, architecture, value.(map[string]CapabilityRecord)}, nil
}

// StaticLLVMCapabilitiesFromDatabase lets offline validation check an input table
// without relying on the table embedded when the validator itself was built.
func StaticLLVMCapabilitiesFromDatabase(db CapabilityDatabase, architecture string) (CompilerCapabilities, error) {
	profile, err := LLVMCapabilityProfileByName(db.Profile)
	if err != nil {
		return nil, err
	}
	architecture, err = normalizeLinuxProbeArchitecture(architecture)
	if err != nil {
		return nil, err
	}
	index, err := capabilityIndex(db, profile)
	if err != nil {
		return nil, err
	}
	return &staticLLVMCapabilities{profile, architecture, index}, nil
}

func capabilityIndex(db CapabilityDatabase, profile LLVMCapabilityProfile) (map[string]CapabilityRecord, error) {
	if err := ValidateCapabilityDatabase(db, profile); err != nil {
		return nil, err
	}
	index := map[string]CapabilityRecord{}
	for _, record := range db.Probes {
		probe, err := normalizeCapabilityProbe(record.CapabilityProbe)
		if err != nil {
			return nil, err
		}
		if _, err := normalizeLinuxProbeArchitecture(probe.Architecture); err != nil {
			return nil, err
		}
		key := probe.Key()
		if old, exists := index[key]; exists && old.Supported != record.Supported {
			return nil, fmt.Errorf("conflicting capability records: %s", key)
		}
		index[key] = record
	}
	return index, nil
}

func (s *staticLLVMCapabilities) Identity() string             { return s.profile.Identity(s.architecture) }
func (s *staticLLVMCapabilities) MinimumClangVersion() Version { return s.profile.MinimumClang }
func (s *staticLLVMCapabilities) MinimumLLDVersion() Version   { return s.profile.MinimumLLD }
func (s *staticLLVMCapabilities) answer(ctx context.Context, probe CapabilityProbe) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	probe.Architecture = s.architecture
	probe, err := normalizeCapabilityProbe(probe)
	if err != nil {
		return false, err
	}
	if record, ok := s.probes[probe.Key()]; ok {
		return record.Supported, nil
	}
	location := ""
	if pos, ok := ctx.Value(capabilityPositionKey{}).(Position); ok {
		location = pos.String() + ": "
	}
	return false, fmt.Errorf("%sunknown LLVM capability probe for %s: %s; collect and validate this probe before updating the capability model", location, s.Identity(), probe.Key())
}
func (s *staticLLVMCapabilities) SupportsOption(ctx context.Context, kind string, candidate, flags []string) (bool, error) {
	return s.answer(ctx, CapabilityProbe{Kind: kind, Candidate: candidate, Context: flags})
}
func (s *staticLLVMCapabilities) SupportsSource(ctx context.Context, language, source string, flags []string) (bool, error) {
	return s.answer(ctx, CapabilityProbe{Kind: "source", Language: language, Source: source, Context: flags})
}

// MeasuredLLVMCapabilities is an offline adapter; repository generation never
// constructs it. Actual measured versions are useful to validators only.
type MeasuredLLVMCapabilities struct{ Probe *LinuxToolProbe }

func (m MeasuredLLVMCapabilities) Identity() string { return m.Probe.Identity() }
func versionFromCode(n int) Version                 { return Version{n / 10000, n / 100 % 100, n % 100} }
func (m MeasuredLLVMCapabilities) MinimumClangVersion() Version {
	return versionFromCode(m.Probe.clangCode)
}
func (m MeasuredLLVMCapabilities) MinimumLLDVersion() Version {
	return versionFromCode(m.Probe.lldCode)
}
func (m MeasuredLLVMCapabilities) SupportsOption(ctx context.Context, kind string, candidate, flags []string) (bool, error) {
	p, err := normalizeCapabilityProbe(CapabilityProbe{Kind: kind, Candidate: candidate, Context: flags})
	if err != nil {
		return false, err
	}
	if kind == "powerpc_script" {
		return m.Probe.supportsPowerPCCompilerScript(ctx, p.Candidate[0], p.Candidate[1])
	}
	return m.Probe.SupportsOption(ctx, kind, p.Candidate, p.Context)
}
func (m MeasuredLLVMCapabilities) SupportsSource(ctx context.Context, language, source string, flags []string) (bool, error) {
	p, err := normalizeCapabilityProbe(CapabilityProbe{Kind: "source", Language: language, Source: source, Context: flags})
	if err != nil {
		return false, err
	}
	return m.Probe.SupportsSource(ctx, language, p.Context, source)
}

func probeTargetProfile(name string) (LinuxTargetProfile, error) {
	// Retain coverage for the internal-only architecture fixtures without
	// exposing additional public image target profiles.
	switch name {
	case "riscv64":
		return LinuxTargetProfile{Name: name, Arch: "riscv", Srcarch: "riscv", TargetTriple: "riscv64-linux-gnu"}, nil
	case "ppc64le":
		return LinuxTargetProfile{Name: name, Arch: "powerpc", Srcarch: "powerpc", TargetTriple: "powerpc64le-linux-gnu"}, nil
	default:
		return LinuxTargetProfileByName(name)
	}
}
