"""Analysis coverage for the configured compiler and prerequisite action edges."""

load("@bazel_skylib//lib:unittest.bzl", "analysistest", "asserts")
load("//internal:kconfig.bzl", "kconfig_file")
load("//internal:linux_objects.bzl", "LinuxConfigInfo", "linux_compile_environment_index", "linux_resolved_config")
load("//internal:llvm_capabilities.bzl", "LinuxCompilerCapabilityInfo", "linux_compiler_check")

visibility("private")

def _compiler_check_test_impl(ctx):
    env = analysistest.begin(ctx)
    target = analysistest.target_under_test(env)
    checks = [action for action in analysistest.target_actions(env) if action.mnemonic == "LinuxCompilerCheck"]
    asserts.equals(env, 1, len(checks))
    if checks:
        check = checks[0]
        asserts.true(env, "clang" in check.argv[0], "check must use the configured compiler")
        asserts.true(env, "-c" in check.argv)
        asserts.true(env, "-ffreestanding" in check.argv)
        asserts.equals(env, 1, len(check.outputs.to_list()))
    asserts.equals(env, "llvm-22", target[LinuxCompilerCapabilityInfo].profile)
    return analysistest.end(env)

_compiler_check_test = analysistest.make(_compiler_check_test_impl)

def _prerequisite_test_impl(ctx):
    env = analysistest.begin(ctx)
    actions = [action for action in analysistest.target_actions(env) if action.mnemonic == ctx.attr.mnemonic]
    asserts.true(env, len(actions) > 0, "missing config materialization actions")
    for action in actions:
        asserts.true(env, any([f.basename == ctx.attr.check_output for f in action.inputs.to_list()]), "config action must wait for the compiler assertion")
    target = analysistest.target_under_test(env)
    if LinuxConfigInfo in target:
        asserts.true(env, any([f.basename == ctx.attr.check_output for f in target[LinuxConfigInfo].files.to_list()]))
    return analysistest.end(env)

_prerequisite_test = analysistest.make(
    _prerequisite_test_impl,
    attrs = {"mnemonic": attr.string(), "check_output": attr.string()},
)

def llvm_capabilities_test_suite(name):
    native.genrule(
        name = name + "_source",
        outs = [name + ".c"],
        tools = ["//internal/cmd/kconfig_parse"],
        cmd = "$(location //internal/cmd/kconfig_parse) -llvm_capability_profile llvm-22 -compiler_check_out $@",
        tags = ["manual"],
    )
    check = name + "_check"
    linux_compiler_check(
        name = check,
        src = ":" + name + "_source",
        profile = "llvm-22",
        compiler_version_text = "clang version 22.1.0, LLD 22.1.0",
        tags = ["manual"],
    )
    _compiler_check_test(name = name + "_configured_compiler_test", target_under_test = ":" + check)
    kconfig_file(name = name + "_fragment", config = "//tests/compact:base.config")
    linux_resolved_config(
        name = name + "_resolved",
        root = "//tests/compact:content_graph.Kconfig",
        config = ":" + name + "_fragment",
        compiler_check = ":" + check,
        llvm_capability_profile = "llvm-22",
        env = {"ARCH": "x86"},
        tags = ["manual"],
    )
    payload_id = "1" * 64
    environment_id = "2" * 64
    linux_compile_environment_index(
        name = name + "_index",
        arch = "x86",
        compiler_check = ":" + check,
        expected_abi = "test/llvm-22/capabilities-v2/x86_64",
        config_payloads = {payload_id: "CONFIG_X86_64=y\n"},
        compile_environments = {environment_id: json.encode({
            "abi": "test/llvm-22/capabilities-v2/x86_64",
            "config_payload": payload_id,
            "generated_header_families": [],
        })},
        tags = ["manual"],
    )
    for suffix, mnemonic in {"resolved": "LinuxResolvedConfig", "index": "LinuxConfigPayloads"}.items():
        _prerequisite_test(
            name = name + "_" + suffix + "_prerequisite_test",
            target_under_test = ":" + name + "_" + suffix,
            check_output = check + ".o",
            mnemonic = mnemonic,
        )
    native.test_suite(name = name, tests = [":" + name + suffix for suffix in ["_configured_compiler_test", "_resolved_prerequisite_test", "_index_prerequisite_test"]])
