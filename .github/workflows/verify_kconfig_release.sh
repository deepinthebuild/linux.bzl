#!/usr/bin/env bash
set -euo pipefail

# A manually published, integrity-pinned release must not be overwritten by
# the workflow triggered when its tag was created. Verify it instead.
release_dir="$(mktemp -d "${RUNNER_TEMP}/kconfig-release.XXXXXX")"
if ! gh api "repos/${GITHUB_REPOSITORY}/releases/tags/${GITHUB_REF_NAME}" \
  --jq '.draft' >"${release_dir}/draft" 2>"${release_dir}/error"; then
  case "$(cat "${release_dir}/error")" in
    *"(HTTP 404)"*) exit 0 ;;
    *) cat "${release_dir}/error" >&2; exit 1 ;;
  esac
fi
if [[ "$(cat "${release_dir}/draft")" != "false" ]]; then
  exit 0
fi

gh release download "${GITHUB_REF_NAME}" --repo "${GITHUB_REPOSITORY}" \
  --dir "${release_dir}" --pattern '*.tar.zst' \
  --pattern SHA256SUMS --pattern kconfig_tool_releases.metadata
(
  cd "${release_dir}"
  sha256sum --check SHA256SUMS
)
cmp dist/SHA256SUMS "${release_dir}/SHA256SUMS"
cmp dist/kconfig_tool_releases.metadata "${release_dir}/kconfig_tool_releases.metadata"
echo 'exists=true' >>"${GITHUB_OUTPUT}"
echo 'Published generator artifacts match this build; keeping the release unchanged.'
