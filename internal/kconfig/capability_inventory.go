package kconfig

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
)

type capabilityPositionKey struct{}

// RecordingCapabilities records the actual expanded probes consumed by both
// parsers. A measured backend is used only by the offline collection command.
type RecordingCapabilities struct {
	CompilerCapabilities
	Architecture string
	Root         string
	mu           sync.Mutex
	records      map[string]CapabilityRecord
}

func (r *RecordingCapabilities) record(ctx context.Context, p CapabilityProbe, supported bool, err error) (bool, error) {
	if err != nil {
		return false, err
	}
	p.Architecture = r.Architecture
	p, err = normalizeCapabilityProbe(p)
	if err != nil {
		return false, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.records == nil {
		r.records = map[string]CapabilityRecord{}
	}
	key := p.Key()
	record := r.records[key]
	record.CapabilityProbe, record.Supported = p, supported
	if pos, ok := ctx.Value(capabilityPositionKey{}).(Position); ok {
		location := strings.TrimPrefix(pos.String(), strings.TrimRight(r.Root, "/")+"/")
		if !slices.Contains(record.Locations, location) {
			record.Locations = append(record.Locations, location)
			slices.Sort(record.Locations)
		}
	}
	if slices.Contains(p.Candidate, "-march=native") {
		record.Reason = "Host-dependent native CPU selection is deliberately disabled."
	}
	r.records[key] = record
	return supported, nil
}
func (r *RecordingCapabilities) SupportsOption(ctx context.Context, kind string, candidate, flags []string) (bool, error) {
	supported, err := r.CompilerCapabilities.SupportsOption(ctx, kind, candidate, flags)
	return r.record(ctx, CapabilityProbe{Kind: kind, Candidate: candidate, Context: flags}, supported, err)
}
func (r *RecordingCapabilities) SupportsSource(ctx context.Context, language, source string, flags []string) (bool, error) {
	supported, err := r.CompilerCapabilities.SupportsSource(ctx, language, source, flags)
	return r.record(ctx, CapabilityProbe{Kind: "source", Language: language, Source: source, Context: flags}, supported, err)
}
func (r *RecordingCapabilities) Records() []CapabilityRecord {
	r.mu.Lock()
	defer r.mu.Unlock()
	keys := make([]string, 0, len(r.records))
	for key := range r.records {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	out := make([]CapabilityRecord, 0, len(keys))
	for _, key := range keys {
		out = append(out, r.records[key])
	}
	return out
}

// LLVMProbeInventory seeds offline validation from the pre-existing model. Every answer is measured separately
// for each reference compiler. Real-source collection extends this inventory.
func LLVMProbeInventory(architecture string) []CapabilityProbe {
	var out []CapabilityProbe
	add := func(kind string, candidate, flags []string) {
		out = append(out, CapabilityProbe{Architecture: architecture, Kind: kind, Candidate: candidate, Context: flags})
	}
	ccMaps := [][]string{linuxLLVMKconfigCCOptionsCommon}
	switch architecture {
	case "x86_64":
		ccMaps = append(ccMaps, linuxLLVMKconfigCCOptionsX86)
	case "aarch64":
		ccMaps = append(ccMaps, linuxLLVMKconfigCCOptionsARM64)
	case "armv7":
		ccMaps = append(ccMaps, linuxLLVMKconfigCCOptionsARMV7)
	case "riscv64":
		ccMaps = append(ccMaps, linuxLLVMKconfigCCOptionsRISCV64)
	case "ppc64le":
		ccMaps = append(ccMaps, linuxLLVMKconfigCCOptionsPPC64LE)
	}
	for _, table := range ccMaps {
		for _, key := range table {
			add("cc_option", strings.Split(key, "\x00"), nil)
		}
	}
	for _, candidate := range [][]string{{"-m32"}, {"-m64"}, {"-fpatchable-function-entry=8"}, {"-mtp=cp15", "-mstack-protector-guard=tls", "-mstack-protector-guard-offset=0"}} {
		add("cc_option", candidate, nil)
	}
	for _, key := range linuxLLVMKconfigLDOptions {
		add("ld_option", strings.Fields(key), nil)
	}
	if architecture == "riscv64" {
		for _, key := range linuxLLVMKconfigLDOptionsRISCV64 {
			add("ld_option", strings.Fields(key), nil)
		}
	}
	tables := [][]string{linuxLLVMKbuildCommonOptions}
	if architecture == "x86_64" {
		tables = append(tables, linuxLLVMKbuildX86Options)
	}
	if architecture == "aarch64" {
		tables = append(tables, linuxLLVMKbuildARM64Options)
	}
	for _, table := range tables {
		for _, key := range table {
			parts := strings.Split(key, "\x00")
			add(parts[0], parts[1:], nil)
		}
	}
	add("cc_option", []string{"-fmacro-prefix-map=/__linux_bzl_source__/="}, nil)
	if architecture == "x86_64" {
		for _, candidate := range linuxLLVMKbuildX86ContextCandidates {
			for _, flags := range [][]string{{"-m32", "-mstack-alignment=4"}, {"-m32", "-mpreferred-stack-boundary=2"}, {"-mstack-alignment=4"}, {"-mpreferred-stack-boundary=2"}} {
				add("cc_option", []string{candidate}, flags)
			}
		}
		for _, flags := range [][]string{{"-m", "elf_x86_64"}, {"--no-ld-generated-unwind-info"}} {
			add("ld_option", []string{"--no-dynamic-linker"}, flags)
		}
	}
	if architecture == "ppc64le" {
		for _, script := range []string{"gcc-check-mprofile-kernel.sh", "gcc-check-fpatchable-function-entry.sh"} {
			for _, endian := range []string{"-mlittle-endian", "-mbig-endian"} {
				add("powerpc_script", []string{script, endian}, nil)
			}
		}
	}
	slices.SortFunc(out, func(a, b CapabilityProbe) int { return strings.Compare(a.Key(), b.Key()) })
	return out
}

func MeasureCapability(ctx context.Context, capabilities CompilerCapabilities, probe CapabilityProbe) (bool, error) {
	if probe.Kind == "source" {
		return capabilities.SupportsSource(ctx, probe.Language, probe.Source, probe.Context)
	}
	return capabilities.SupportsOption(ctx, probe.Kind, probe.Candidate, probe.Context)
}

func LinuxProbeEnvironment(capabilities CompilerCapabilities) map[string]string {
	return map[string]string{
		"AR": "llvm-ar", "BINDGEN": "bindgen", "CC": "clang", "CC_VERSION_TEXT": "clang version " + capabilities.MinimumClangVersion().String(),
		"CLANG_FLAGS": "-fintegrated-as", "LD": "ld.lld", "NM": "llvm-nm", "OBJCOPY": "llvm-objcopy", "PAHOLE": "pahole", "PYTHON3": "python3", "RUSTC": "rustc",
	}
}

func ValidateCapabilityDatabase(db CapabilityDatabase, profile LLVMCapabilityProfile) error {
	if db.Profile != profile.Name || db.ReferenceClang != profile.MinimumClang || db.ReferenceLLD != profile.MinimumLLD {
		return fmt.Errorf("database references must match %s minimum versions", profile.Name)
	}
	if len(db.Probes) == 0 || !strings.HasPrefix(db.ReferenceIdentity, "sha256-") {
		return fmt.Errorf("database %s has no measured reference data", profile.Name)
	}
	return nil
}

// CollectLinuxCapabilities evaluates the maintained Kconfig/Kbuild dialect using
// an explicit backend. It never runs Linux Makefiles or shell scripts.
func CollectLinuxCapabilities(ctx context.Context, root, architecture string, configs []map[string]string, capabilities CompilerCapabilities) error {
	profile, err := probeTargetProfile(architecture)
	if err != nil {
		return err
	}
	vars := map[string]string{"ARCH": profile.Arch, "SRCARCH": profile.Srcarch, "UTS_MACHINE": profile.UTSMachine, "srctree": root, "RUSTC_VERSION_TEXT": "rustc 1.97.0", "CFLAGS_UBSAN_TRAP": "-fsanitize-trap=undefined", "PROFILING": ""}
	if architecture != "armv7" {
		vars["BITS"] = "64"
	} else {
		vars["BITS"] = "32"
	}
	if architecture == "aarch64" || architecture == "armv7" {
		for _, name := range []string{"ARCH_CORE", "ARCH_DRIVERS", "CC_FLAGS_FTRACE", "CC_FLAGS_LTO", "CC_FLAGS_SCS", "DISABLE_KSTACK_ERASE", "DISABLE_LATENT_ENTROPY_PLUGIN"} {
			vars[name] = ""
		}
		vars["CC_FLAGS_FPU"] = "-ffreestanding -D_LINUX_FPU_COMPILATION_UNIT"
		vars["CC_FLAGS_NO_FPU"] = "-mgeneral-regs-only"
	}
	shell, err := LinuxProbeShellWithCapabilities(architecture, capabilities, LinuxProbeDefaultRustcVersion, LinuxProbeDefaultRustcLLVMVersion)
	if err != nil {
		return err
	}
	env := LinuxProbeEnvironment(capabilities)
	env["ARCH"], env["SRCARCH"] = profile.Arch, profile.Srcarch
	tree, err := ParseFile(ctx, filepath.Join(root, "Kconfig"), Options{RootDir: root, Variables: vars, Env: env, AllowShell: true, Shell: shell})
	if err != nil {
		return err
	}
	if len(configs) == 0 {
		configs = []map[string]string{{}}
	}
	for index, raw := range configs {
		prepared, err := profile.PrepareTargetConfig(raw)
		if err != nil {
			return err
		}
		resolved, err := tree.ResolveConfig(fmt.Sprintf("capability-corpus-%d", index), prepared)
		if err != nil {
			return err
		}
		kbvars := cloneStringMap(vars)
		kbvars["comma"] = ","
		for name, value := range resolved.Effective {
			if !resolved.ShouldWrite(name) || value == "n" {
				kbvars[name] = ""
				continue
			}
			if len(value) >= 2 && value[0] == '"' && value[len(value)-1] == '"' {
				value = value[1 : len(value)-1]
			}
			kbvars[name] = value
		}
		_, err = ParseKbuildDirectoryTree(filepath.Join(root, "Kbuild"), KbuildOptions{RootDir: root, RootMakefiles: []string{filepath.Join("arch", profile.Srcarch, "Makefile")}, Variables: kbvars, ConfigVariablesComplete: true, Capabilities: capabilities})
		if err != nil {
			return fmt.Errorf("config %d: %w", index, err)
		}
	}
	return nil
}

func ReadCapabilityConfig(path string) (map[string]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return ParseConfig(file)
}
