"""Bazel enforcement of the compiler contract selected during generation."""

load("@rules_cc//cc:action_names.bzl", "C_COMPILE_ACTION_NAME")
load("@rules_cc//cc:find_cc_toolchain.bzl", "find_cpp_toolchain", "use_cc_toolchain")
load("@rules_cc//cc/common:cc_common.bzl", "cc_common")
load(":path_mapping.bzl", "path_mapped_run")

visibility("public")

LinuxCompilerCapabilityInfo = provider(fields = {
    "profile": "Selected static capability profile.",
    "compiler_version_text": "Modeled minimum Clang/LLD identity.",
    "files": "Compiler check artifacts required by kernel actions.",
})

def _linux_compiler_check_impl(ctx):
    cc_toolchain = find_cpp_toolchain(ctx)
    features = cc_common.configure_features(
        ctx = ctx,
        cc_toolchain = cc_toolchain,
        requested_features = ctx.features,
        unsupported_features = ctx.disabled_features,
    )
    compiler = cc_common.get_tool_for_action(
        feature_configuration = features,
        action_name = C_COMPILE_ACTION_NAME,
    )
    for name in ["ld.lld", "llvm-ar", "llvm-nm", "llvm-objcopy"]:
        matches = [f for f in cc_toolchain.all_files.to_list() if f.basename in [name, name + ".exe"]]
        if len(matches) != 1:
            fail("%s requires the selected C++ toolchain to expose exactly one %s; found %d" % (ctx.attr.profile, name, len(matches)))
    output = ctx.actions.declare_file(ctx.label.name + ".o")
    variables = cc_common.create_compile_variables(
        feature_configuration = features,
        cc_toolchain = cc_toolchain,
        user_compile_flags = ctx.fragments.cpp.copts + ctx.fragments.cpp.conlyopts,
    )
    args = ctx.actions.args()
    args.add_all(cc_common.get_memory_inefficient_command_line(
        feature_configuration = features,
        action_name = C_COMPILE_ACTION_NAME,
        variables = variables,
    ))
    args.add_all(["-ffreestanding", "-c"])
    args.add(ctx.file.src)
    args.add("-o", output)
    path_mapped_run(
        ctx.actions,
        executable = compiler,
        inputs = depset([ctx.file.src], transitive = [cc_toolchain.all_files]),
        outputs = [output],
        arguments = [args],
        env = cc_common.get_environment_variables(
            feature_configuration = features,
            action_name = C_COMPILE_ACTION_NAME,
            variables = variables,
        ),
        mnemonic = "LinuxCompilerCheck",
        progress_message = "Checking Linux LLVM capability floor %{label}",
    )
    files = depset([output])
    return [
        DefaultInfo(files = files),
        LinuxCompilerCapabilityInfo(
            profile = ctx.attr.profile,
            compiler_version_text = ctx.attr.compiler_version_text,
            files = files,
        ),
    ]

linux_compiler_check = rule(
    implementation = _linux_compiler_check_impl,
    attrs = {
        "src": attr.label(allow_single_file = [".c"], mandatory = True),
        "profile": attr.string(mandatory = True),
        "compiler_version_text": attr.string(mandatory = True),
    },
    fragments = ["cpp"],
    toolchains = use_cc_toolchain(),
)
