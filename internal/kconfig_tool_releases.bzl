"""Release metadata for prebuilt kconfig repository-rule tools.

Each archive must extract the requested host executables at its root.
"""

visibility("//...")

KCONFIG_TOOL_VERSION = "v0.0.25-llvm-capabilities.1"

_RELEASE_BASE_URL = "https://github.com/deepinthebuild/linux.bzl/releases/download/kconfig-{version}".format(
    version = KCONFIG_TOOL_VERSION,
)

KCONFIG_TOOL_RELEASES = {
    "darwin_amd64": struct(
        integrity = "sha256-PLqZangZosWb0b/RCbAD5BELLfi88xL+J7zImrKmZNw=",
        urls = ["{}/kconfig-darwin-amd64.tar.zst".format(_RELEASE_BASE_URL)],
    ),
    "darwin_arm64": struct(
        integrity = "sha256-kTpUp/XF5rlaOOzZZ3v0oAhxQYFmsK/d7FhQY3OPm+o=",
        urls = ["{}/kconfig-darwin-arm64.tar.zst".format(_RELEASE_BASE_URL)],
    ),
    "linux_amd64": struct(
        integrity = "sha256-mb4Mpxb9woDk0P89nJB4rN/4uz8TiK5Xp1HcbxnvkP4=",
        urls = ["{}/kconfig-linux-amd64.tar.zst".format(_RELEASE_BASE_URL)],
    ),
    "linux_arm64": struct(
        integrity = "sha256-gejUk0i3OXwCtxZBzvWkVxEnBrTA777E+IPNsn8xvkA=",
        urls = ["{}/kconfig-linux-arm64.tar.zst".format(_RELEASE_BASE_URL)],
    ),
    "windows_amd64": struct(
        integrity = "sha256-RurDimKUv38zgM2+++/klIzb8fmUenyEFAFiG9t+7xM=",
        urls = ["{}/kconfig-windows-amd64.tar.zst".format(_RELEASE_BASE_URL)],
    ),
    "windows_arm64": struct(
        integrity = "sha256-dWfTp1Hcrq37iqyq6/gJy5Zc4HTQNnF6QjkkFUgRLSs=",
        urls = ["{}/kconfig-windows-arm64.tar.zst".format(_RELEASE_BASE_URL)],
    ),
}
