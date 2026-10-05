#!/usr/bin/env python3
"""Generate or validate a profile against supported sources and config fixtures.

Compiler paths are explicit and used only by this offline maintenance command.
The checked-in database is written only in generate mode, using the exact floor.
"""
import argparse
import pathlib
import re
import json
import subprocess
import tempfile

REPO = pathlib.Path(__file__).resolve().parent.parent
CONFIGS = {
    "x86_64": ["e2e/tiny.config", "e2e/kvm.config", "e2e/rust.config", "examples/configs/x86_64.config"],
    "aarch64": ["e2e/tiny_arm64.config", "e2e/kvm_arm64.config", "e2e/rust_arm64.config", "examples/configs/aarch64.config"],
    "armv7": ["e2e/armv7.config"],
}
OVERLAYS = ["e2e/modversions.config", "e2e/srcversions.config", "e2e/verity.config", "e2e/cnic.config", "examples/configs/btf.config", "examples/configs/debug.config", "examples/configs/lz4.config"]


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--mode", choices=["generate", "validate"], default="validate")
    parser.add_argument("--generator", required=True, type=pathlib.Path)
    parser.add_argument("--profile", default="llvm-22")
    parser.add_argument("--clang", required=True, type=pathlib.Path)
    parser.add_argument("--lld", required=True, type=pathlib.Path)
    parser.add_argument("--source", action="append", required=True, type=pathlib.Path)
    args = parser.parse_args()
    database = REPO / "internal/kconfig/llvm_capabilities" / (args.profile + ".json")
    command = [str(args.generator.resolve()), "-mode", args.mode, "-profile", args.profile,
               "-clang", str(args.clang.absolute()), "-lld", str(args.lld.absolute())]
    if args.mode == "validate" or json.loads(database.read_text()).get("probes"):
        command += ["-input", str(database)]
    if args.mode == "generate":
        command += ["-out", str(database), "-corpus", str(REPO / "internal/kconfig/testdata/llvm_probe_corpus.json")]
    for source in args.source:
        command += ["-source", str(source.resolve())]
    with tempfile.TemporaryDirectory(prefix="linux-bzl-capabilities-") as directory:
        empty = pathlib.Path(directory) / "default.config"
        empty.write_text("")
        for arch, configs in CONFIGS.items():
            command += ["-config", f"{arch}={empty}"]
            for config in configs:
                command += ["-config", f"{arch}={REPO / config}"]
            for index, overlay in enumerate(OVERLAYS):
                if arch != "x86_64" and overlay.startswith("e2e/"):
                    continue
                fragment = pathlib.Path(directory) / f"{arch}-{index}.config"
                lines = {}
                for path in [configs[0], overlay]:
                    for line in (REPO / path).read_text().splitlines():
                        match = re.match(r"(?:# )?(CONFIG_[A-Za-z0-9_]+)(?:=| is not set)", line)
                        if match:
                            lines[match[1]] = line
                fragment.write_text("\n".join(lines.values()) + "\n")
                command += ["-config", f"{arch}={fragment}"]
        subprocess.run(command, cwd=REPO, check=True)


if __name__ == "__main__":
    main()
