#!/usr/bin/env python3
"""Fetch a checksum-pinned upstream LLVM reference for offline profile validation."""
import argparse
import hashlib
import json
import pathlib
import subprocess
import urllib.request


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("version")
    parser.add_argument("destination", type=pathlib.Path)
    args = parser.parse_args()
    references = json.loads(pathlib.Path(__file__).with_name("llvm_references.json").read_text())
    reference = references[args.version]
    destination = args.destination.resolve()
    destination.mkdir(parents=True, exist_ok=True)
    archive = destination / "reference.tar.xz"
    if not archive.exists():
        urllib.request.urlretrieve(reference["url"], archive)
    with archive.open("rb") as stream:
        digest = hashlib.file_digest(stream, "sha256").hexdigest()
    if digest != reference["sha256"]:
        raise SystemExit(f"LLVM {args.version} checksum mismatch")
    subprocess.run(["tar", "-xJf", str(archive), "-C", str(destination), "--strip-components=1",
                    "--wildcards", "*/bin/clang*", "*/bin/lld", "*/bin/ld.lld",
                    "*/lib/libLLVM*", "*/lib/libclang*", "*/lib/clang/*"], check=True)
    print(destination)


if __name__ == "__main__":
    main()
