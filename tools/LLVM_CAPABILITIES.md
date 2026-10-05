# Maintaining LLVM capability profiles

Normal repository generation uses only the embedded static database. These
commands are offline maintenance tools; they are never called by repository
rules. `llvm_profiles.json` is the authoritative contract: explicit minimum
Clang/LLD versions and a capability-model revision, independent of packaging.

The current released-major profiles are llvm-19 through llvm-23, defaulting to
llvm-22. Each initially uses its X.1.0 floor. Raise a floor for a required patch
fix, bump the model revision, and regenerate the table at the new exact floor.
Create multiple profiles within a major only if distinct simultaneous capability
surfaces are needed. Keep the profile manifest, tables, reference pins, workflow
matrix, and profile tests consistent when adding a major or changing a floor.

## Generate and validate

Build the maintenance tool and obtain Linux 6.12.96 and 6.18.39 sources. Reference
archives in `llvm_references.json` are upstream LLVM Linux X64 releases pinned
by SHA-256, independent of Hermetic LLVM. The binaries require the corresponding
runtime libraries (the CI runner supplies libxml2 and ICU 70).

```sh
go build -o /tmp/llvm_capabilities ./internal/cmd/llvm_capabilities
python3 tools/fetch_llvm_reference.py 22.1.0 /tmp/llvm-22.1.0
python3 tools/llvm_capabilities.py --mode generate --profile llvm-22 \
  --generator /tmp/llvm_capabilities \
  --clang /tmp/llvm-22.1.0/bin/clang --lld /tmp/llvm-22.1.0/bin/ld.lld \
  --source /path/to/linux-6.12.96 --source /path/to/linux-6.18.39
```

The collector evaluates Kconfig and active Kbuild files for the maintained
architecture/config/overlay matrix in `llvm_capabilities.py`. It records fully
expanded probes, ordered compiler context, source locations, and measured
answers. The option inventory preserves previously supported probe spellings;
it contains no capability answers. The regression corpus retains additional
exact probes exercised by parser tests. Neither list infers support from a
version number. Probes from the previous database are remeasured so temporarily
unreachable probes are not lost. Only the exact profile floor can generate data.

Review the generated JSON diff and bump `model_revision` whenever changing an
existing contract's floors or answers. Use `-config architecture=fragment` and
`-source` directly with the Go tool to collect additional configurations or
kernel versions. New probes must be collected and validated explicitly; an
unrecognized probe is an error, not an assumed unsupported feature.

Replace `--mode generate` with `--mode validate` to compare the table against a
reference toolchain without writing it. At the exact floor, every answer must
match (except documented conservative policy, such as disabling `-march=native`).
Later versions must preserve every promised true capability; additional support
is permitted and leaves the generated graph unchanged. Source collection in
validation mode also checks that the database covers the maintained fixtures,
using the profile's floor for Linux version tests even with a newer compiler.

The CI matrix checks each floor and a pinned later patch: 19.1.7, 20.1.8, 21.1.8,
22.1.8, and 23.1.2. It also validates llvm-22 with LLVM 23.1.2. Add intermediate references or newer releases to the matrix
when investigating a fix or regression. For a custom installation, run the
validator directly against its clang/ld.lld. A failed guaranteed capability is
an incompatibility to investigate and document; never silently rewrite the
profile around a later compiler. Tests of a finite set of releases are evidence,
not a claim that all future releases cannot regress.

## Repository-generation regression test

`tests/static_generation` is a separate consuming module with a standard Bazel
Linux/x86_64, aarch64, and armv7 platforms and no C++ toolchain registration. It verifies that generating
a real Linux graph requires no repository-time compiler installation:

```sh
go build -o /tmp/kconfig_parse ./internal/cmd/kconfig_parse
(cd tests/static_generation && bazel query \
  "set(@graph//graph:metadata.json @graph_6_12//graph:metadata.json @graph_arm64//graph:metadata.json @graph_armv7//graph:metadata.json)" \
  --repo_env=LINUX_BZL_KCONFIG_PARSE=/tmp/kconfig_parse)
```

Go tests exercise exact source/context matching, encoded floors, and profile
identity. Set `LINUX_BZL_TEST_CLANG=/absolute/path/to/clang` to also compile the
assertion with simulated below/at/above-floor version macros. Bazel analysis
tests verify configured-toolchain selection and prerequisite edges; build
`//internal/tests:llvm_capabilities_test_check` to execute the generated assertion
using the real resolved toolchain. The source is generated from the same profile
manifest as the repository's assertion.
