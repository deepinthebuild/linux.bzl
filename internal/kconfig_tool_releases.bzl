"""Release metadata for prebuilt kconfig repository-rule tools.

Each archive must extract the requested host executables at its root.
"""

visibility("//...")

KCONFIG_TOOL_VERSION = "v0.0.25-llvm-capabilities.2"

_RELEASE_BASE_URL = "https://github.com/deepinthebuild/linux.bzl/releases/download/kconfig-{version}".format(
    version = KCONFIG_TOOL_VERSION,
)

KCONFIG_TOOL_RELEASES = {
    "darwin_amd64": struct(
        integrity = "sha256-1ASx7r/JCWpkPhsr2RU1f7kG70ynVPzoZfmHXCySCM0=",
        urls = ["{}/kconfig-darwin-amd64.tar.zst".format(_RELEASE_BASE_URL)],
    ),
    "darwin_arm64": struct(
        integrity = "sha256-1ekvCRKNHS3BPzVGpyHoeeBSQ6Un7dMMAe0mt2M8Fns=",
        urls = ["{}/kconfig-darwin-arm64.tar.zst".format(_RELEASE_BASE_URL)],
    ),
    "linux_amd64": struct(
        integrity = "sha256-dqWHeH6EWw9ZyVSgP59wqLqrowNccumh3+Qa2rSm1fk=",
        urls = ["{}/kconfig-linux-amd64.tar.zst".format(_RELEASE_BASE_URL)],
    ),
    "linux_arm64": struct(
        integrity = "sha256-TEACpSmBpmhWrto/4CrWUhdcIXV+I3UHf870+PxrtZE=",
        urls = ["{}/kconfig-linux-arm64.tar.zst".format(_RELEASE_BASE_URL)],
    ),
    "windows_amd64": struct(
        integrity = "sha256-quIgLGHleZeTPeHM12z6yl103SHiytwLydCP5N63XgA=",
        urls = ["{}/kconfig-windows-amd64.tar.zst".format(_RELEASE_BASE_URL)],
    ),
    "windows_arm64": struct(
        integrity = "sha256-+uS0JMXBDIocC2cp8xzHhyDbR41LwjSUjpEMRU8jzD0=",
        urls = ["{}/kconfig-windows-arm64.tar.zst".format(_RELEASE_BASE_URL)],
    ),
}
