package kconfig

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestStaticLLVMProfilesUseConservativeVersionsWithoutTools(t *testing.T) {
	t.Setenv("PATH", "")
	for major := 19; major <= 23; major++ {
		name := fmt.Sprintf("llvm-%d", major)
		for _, arch := range []string{"x86_64", "aarch64", "armv7", "riscv64", "ppc64le"} {
			t.Run(name+"/"+arch, func(t *testing.T) {
				capabilities, err := StaticLLVMCapabilities(name, arch)
				if err != nil {
					t.Fatal(err)
				}
				if got := capabilities.Identity(); got != name+"/capabilities-v1/"+arch {
					t.Fatal(got)
				}
				if got := capabilities.MinimumClangVersion().Encoded(); got != major*10000+100 {
					t.Fatal(got)
				}
				if capabilities.MinimumClangVersion() != capabilities.MinimumLLDVersion() {
					t.Fatal("unexpected LLD floor")
				}
				shell, err := LinuxProbeShellWithCapabilities(arch, capabilities, LinuxProbeDefaultRustcVersion, LinuxProbeDefaultRustcLLVMVersion)
				if err != nil {
					t.Fatal(err)
				}
				for command, prefix := range map[string]string{"scripts/cc-version.sh clang": "Clang", "scripts/ld-version.sh ld.lld": "LLD"} {
					got, err := shell(context.Background(), command)
					if want := fmt.Sprintf("%s %d", prefix, major*10000+100); err != nil || got != want {
						t.Fatalf("got %q, %v; want %q", got, err, want)
					}
				}
				if _, err := capabilities.SupportsOption(context.Background(), "cc_option", []string{"-fno-addrsig"}, nil); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

func TestStaticLLVMProbeKeysAreExact(t *testing.T) {
	capabilities, err := StaticLLVMCapabilities("llvm-22", "x86_64")
	if err != nil {
		t.Fatal(err)
	}
	source := "int __seg_fs fs; int __seg_gs gs;"
	if ok, err := capabilities.SupportsSource(context.Background(), "c", source, nil); err != nil || !ok {
		t.Fatalf("known source: %v %v", ok, err)
	}
	for _, modified := range []string{source + " invalid code", "/* " + source + " */ int broken = ;"} {
		if _, err := capabilities.SupportsSource(context.Background(), "c", modified, nil); err == nil {
			t.Fatal("accepted unknown source by substring")
		}
	}
	if _, err := capabilities.SupportsOption(context.Background(), "cc_option", []string{"-fno-addrsig"}, []string{"-fbrand-new-context"}); err == nil {
		t.Fatal("ignored unknown context")
	}
	if _, err := StaticLLVMCapabilities("llvm-999", "x86_64"); err == nil {
		t.Fatal("accepted unknown profile")
	}
	if _, err := StaticLLVMCapabilities("llvm-22", "made-up-cpu"); err == nil {
		t.Fatal("accepted unknown architecture")
	}
}

func TestCapabilityDatabaseRejectsChangedFloorsAndConflicts(t *testing.T) {
	data, err := llvmCapabilityFiles.ReadFile("llvm_capabilities/llvm-22.json")
	if err != nil {
		t.Fatal(err)
	}
	var db CapabilityDatabase
	if err := json.Unmarshal(data, &db); err != nil {
		t.Fatal(err)
	}
	db.ReferenceClang.Patch++
	if _, err := StaticLLVMCapabilitiesFromDatabase(db, "x86_64"); err == nil {
		t.Fatal("accepted stale floor")
	}
	db.ReferenceClang.Patch--
	conflict := db.Probes[0]
	conflict.Supported = !conflict.Supported
	db.Probes = append(db.Probes, conflict)
	if _, err := StaticLLVMCapabilitiesFromDatabase(db, "x86_64"); err == nil {
		t.Fatal("accepted conflicting answers")
	}
}

func TestCompilerCheckEncodesFullFloor(t *testing.T) {
	profile, err := LLVMCapabilityProfileByName("llvm-22")
	if err != nil {
		t.Fatal(err)
	}
	profile.MinimumClang.Patch = 4
	source := profile.CompilerCheckSource()
	for _, want := range []string{"#ifndef __clang__", "__clang_major__ * 10000", "__clang_minor__ * 100", "__clang_patchlevel__", "LINUX_BZL_CLANG_VERSION < 220104", "Clang 22.1.4 or newer"} {
		if !strings.Contains(source, want) {
			t.Fatalf("missing %s in %s", want, source)
		}
	}
	// The model revision changes cache identity independently of the version floor.
	before := profile.Identity("x86_64")
	profile.ModelRevision = "capabilities-v2"
	if profile.Identity("x86_64") == before || profile.CompilerCheckSource() != source {
		t.Fatal("model revision coupled to compiler version")
	}
}

func TestCompilerCheckCompilesAtAndAboveFloor(t *testing.T) {
	clang := os.Getenv("LINUX_BZL_TEST_CLANG")
	if clang == "" {
		t.Skip("set LINUX_BZL_TEST_CLANG for offline compiler assertion validation")
	}
	profile, _ := LLVMCapabilityProfileByName("llvm-22")
	profile.MinimumClang.Patch = 4
	dir := t.TempDir()
	source := filepath.Join(dir, "check.c")
	if err := os.WriteFile(source, []byte(profile.CompilerCheckSource()), 0644); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		version Version
		want    bool
	}{{Version{21, 1, 8}, false}, {Version{22, 1, 3}, false}, {Version{22, 1, 4}, true}, {Version{22, 1, 8}, true}, {Version{23, 1, 0}, true}} {
		args := []string{"-ffreestanding", "-Wno-builtin-macro-redefined", "-Wno-macro-redefined", "-U__clang_major__", "-U__clang_minor__", "-U__clang_patchlevel__", fmt.Sprintf("-D__clang_major__=%d", test.version.Major), fmt.Sprintf("-D__clang_minor__=%d", test.version.Minor), fmt.Sprintf("-D__clang_patchlevel__=%d", test.version.Patch), "-c", source, "-o", filepath.Join(dir, "check.o")}
		out, err := exec.Command(clang, args...).CombinedOutput()
		if (err == nil) != test.want {
			t.Fatalf("%s: %v %s", test.version, err, out)
		}
	}
	if out, err := exec.Command(clang, "-U__clang__", "-c", source, "-o", filepath.Join(dir, "check.o")).CombinedOutput(); err == nil || !strings.Contains(string(out), "requires Clang") {
		t.Fatalf("non-Clang check: %v %s", err, out)
	}
}

func TestParseCompilerVersions(t *testing.T) {
	for _, text := range []string{"19.1.0", "22.1.4", "23.1.0"} {
		version, err := ParseVersion(text)
		if err != nil || version.String() != text {
			t.Fatalf("%s: %v", text, err)
		}
	}
	for _, text := range []string{"22", "22.1", "22.1.100", "22.01.0", "0.1.0", "22.1.-1", "22.1.0-rc1"} {
		if _, err := ParseVersion(text); err == nil {
			t.Fatalf("accepted %q", text)
		}
	}
}

func TestKbuildUsesStaticSourceCapabilities(t *testing.T) {
	capabilities, err := StaticLLVMCapabilities("llvm-22", "x86_64")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "Kbuild")
	if err := os.WriteFile(path, []byte("obj-y += test.o\nCFLAGS_test.o := $(call cc-option,-fno-addrsig) $(call as-instr,endbr64,-DHAS_ENDBR,-DNO_ENDBR)\n"), 0644); err != nil {
		t.Fatal(err)
	}
	kb, err := ParseKbuildFileWithOptions(path, KbuildOptions{Variables: map[string]string{"SRCARCH": "x86", "KBUILD_AFLAGS": "-Wa,--fatal-warnings"}, Capabilities: capabilities})
	if err != nil {
		t.Fatal(err)
	}
	var flags []string
	for _, flag := range kb.Flags {
		if flag.Object == "test.o" {
			flags = append(flags, flag.Flags...)
		}
	}
	if got := strings.Join(flags, " "); got != "-fno-addrsig -DHAS_ENDBR" {
		t.Fatalf("static Kbuild flags: %s", got)
	}
}

func TestCapabilityContextPathsDoNotAffectIdentity(t *testing.T) {
	first, err := normalizeCapabilityProbe(CapabilityProbe{Kind: "cc_option", Candidate: []string{"-fno-addrsig"}, Context: []string{"-fmacro-prefix-map=/first/source/=", "-m64"}})
	if err != nil {
		t.Fatal(err)
	}
	second, err := normalizeCapabilityProbe(CapabilityProbe{Kind: "cc_option", Candidate: []string{"-fno-addrsig"}, Context: []string{"-fmacro-prefix-map=/other/source/=", "-m64"}})
	if err != nil {
		t.Fatal(err)
	}
	if first.Key() != second.Key() {
		t.Fatal("repository path leaked into capability identity")
	}
}
