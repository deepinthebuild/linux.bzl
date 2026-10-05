// llvm_capabilities is an offline maintenance tool. It is never invoked by a
// repository rule or by normal kernel graph generation.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/hermeticbuild/linux.bzl/internal/kconfig"
)

type stringsFlag []string

func (s *stringsFlag) String() string         { return strings.Join(*s, ",") }
func (s *stringsFlag) Set(value string) error { *s = append(*s, value); return nil }

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	mode := flag.String("mode", "validate", "generate or validate")
	profileName := flag.String("profile", kconfig.DefaultLLVMCapabilityProfile, "LLVM capability profile")
	clang := flag.String("clang", "", "Reference Clang executable (offline only)")
	lld := flag.String("lld", "", "Reference LLD executable (offline only)")
	input := flag.String("input", "", "Existing JSON database to extend or validate")
	corpus := flag.String("corpus", "", "Additional JSON probe inputs (without answers), for regression fixtures")
	output := flag.String("out", "", "Output JSON database for generate")
	architectures := flag.String("architectures", "x86_64,aarch64,armv7,riscv64,ppc64le", "Comma-separated architectures")
	var sources, configs stringsFlag
	flag.Var(&sources, "source", "Linux source root to collect (repeatable)")
	flag.Var(&configs, "config", "Architecture=fragment path (repeatable)")
	flag.Parse()
	if *mode != "generate" && *mode != "validate" {
		return fmt.Errorf("unknown mode %q", *mode)
	}
	profile, err := kconfig.LLVMCapabilityProfileByName(*profileName)
	if err != nil {
		return err
	}
	db := kconfig.CapabilityDatabase{Profile: profile.Name, ReferenceClang: profile.MinimumClang, ReferenceLLD: profile.MinimumLLD}
	if *input != "" {
		data, err := os.ReadFile(*input)
		if err != nil {
			return err
		}
		if err = json.Unmarshal(data, &db); err != nil {
			return err
		}
		if err = kconfig.ValidateCapabilityDatabase(db, profile); err != nil {
			return err
		}
	}
	if *corpus != "" && *mode == "validate" {
		return fmt.Errorf("-corpus is only supported during generation; validate the resulting database")
	}
	if *corpus != "" {
		data, err := os.ReadFile(*corpus)
		if err != nil {
			return err
		}
		var probes []kconfig.CapabilityProbe
		if err := json.Unmarshal(data, &probes); err != nil {
			return err
		}
		for _, probe := range probes {
			db.Probes = append(db.Probes, kconfig.CapabilityRecord{CapabilityProbe: probe, Locations: []string{"regression corpus"}})
		}
	}
	if *mode == "validate" && *input == "" {
		return fmt.Errorf("validate requires -input")
	}
	if *mode != "validate" && *output == "" {
		return fmt.Errorf("generate requires -out")
	}
	fragments := map[string][]map[string]string{}
	for _, entry := range configs {
		arch, path, ok := strings.Cut(entry, "=")
		if !ok {
			return fmt.Errorf("config requires architecture=path")
		}
		config, err := kconfig.ReadCapabilityConfig(path)
		if err != nil {
			return err
		}
		fragments[arch] = append(fragments[arch], config)
	}
	records := map[string]kconfig.CapabilityRecord{}
	for _, record := range db.Probes {
		records[record.Key()] = record
	}
	ctx := context.Background()
	for _, architecture := range strings.Split(*architectures, ",") {
		probe, err := kconfig.NewLinuxToolProbe(kconfig.LinuxToolProbeOptions{Profile: architecture, ClangPath: *clang, LLDPath: *lld})
		if err != nil {
			return err
		}
		measured := kconfig.MeasuredLLVMCapabilities{Probe: probe}
		if measured.MinimumClangVersion().Encoded() < profile.MinimumClang.Encoded() || measured.MinimumLLDVersion().Encoded() < profile.MinimumLLD.Encoded() {
			return fmt.Errorf("reference toolchain is below %s floors", profile.Name)
		}
		if *mode != "validate" && (measured.MinimumClangVersion() != profile.MinimumClang || measured.MinimumLLDVersion() != profile.MinimumLLD) {
			return fmt.Errorf("generation requires exact minimum reference versions %s/%s", profile.MinimumClang, profile.MinimumLLD)
		}
		if *mode == "validate" {
			strict := measured.MinimumClangVersion() == profile.MinimumClang && measured.MinimumLLDVersion() == profile.MinimumLLD
			for _, record := range db.Probes {
				if record.Architecture != architecture {
					continue
				}
				actual, err := kconfig.MeasureCapability(ctx, measured, record.CapabilityProbe)
				if err != nil {
					return err
				}
				if (record.Supported && !actual) || (strict && actual != record.Supported && record.Reason == "") {
					return fmt.Errorf("capability regression: %s: expected %v, measured %v (locations %v)", record.Key(), record.Supported, actual, record.Locations)
				}
			}
			static, err := kconfig.StaticLLVMCapabilitiesFromDatabase(db, architecture)
			if err != nil {
				return err
			}
			for _, source := range sources {
				if err := kconfig.CollectLinuxCapabilities(ctx, source, architecture, fragments[architecture], static); err != nil {
					return fmt.Errorf("coverage %s %s: %w", architecture, source, err)
				}
			}
			continue
		}
		db.ReferenceIdentity = measured.Identity()
		recorder := &kconfig.RecordingCapabilities{CompilerCapabilities: measured, Architecture: architecture}
		for _, item := range kconfig.LLVMProbeInventory(architecture) {
			if _, err := kconfig.MeasureCapability(ctx, recorder, item); err != nil {
				return fmt.Errorf("inventory %s: %w", item.Key(), err)
			}
		}
		// Replay the previous corpus to retain probes from configurations which
		// became unreachable when a capability answer changed at this floor.
		for _, record := range db.Probes {
			if record.Architecture == architecture {
				if _, err := kconfig.MeasureCapability(ctx, recorder, record.CapabilityProbe); err != nil {
					return err
				}
			}
		}
		for _, source := range sources {
			recorder.Root = source
			if err := kconfig.CollectLinuxCapabilities(ctx, source, architecture, fragments[architecture], recorder); err != nil {
				return fmt.Errorf("collect %s %s: %w", architecture, source, err)
			}
		}
		for _, record := range recorder.Records() {
			if old, ok := records[record.Key()]; ok {
				record.Locations = append(record.Locations, old.Locations...)
				slices.Sort(record.Locations)
				record.Locations = slices.Compact(record.Locations)
			}
			records[record.Key()] = record
		}
		fmt.Fprintf(os.Stderr, "collected %d probes for %s/%s\n", len(recorder.Records()), profile.Name, architecture)
	}
	if *mode == "validate" {
		fmt.Printf("Validated %s against %s and %s\n", profile.Name, *clang, *lld)
		return nil
	}
	db.Probes = nil
	for _, record := range records {
		db.Probes = append(db.Probes, record)
	}
	slices.SortFunc(db.Probes, func(a, b kconfig.CapabilityRecord) int { return strings.Compare(a.Key(), b.Key()) })
	data, err := json.MarshalIndent(db, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(*output, append(data, '\n'), 0644)
}
