#!/usr/bin/env bash
# Builds the reference zygote's template artifact for this machine's
# architecture, with no images and no key.
#
#   hack/release/zygote-artifact.sh <out> <owner>   (inside the dev container)
#
# <out>/artifact is the artifact, with its digest in <out>/artifact/DIGEST.
# <out>/zygotectl is static, so it also runs on the host. Both end up owned
# by <owner> (uid:gid), since the container runs as root.
#
# The zygote is rootfs.sh's, not make zygote's. It is static, so it runs in
# the released images, which hold no libssl, and on the runc backend, which
# takes only a static bare template. zygotectl build runs it to READY once
# as a smoke test.
set -euo pipefail
cd "$(dirname "$0")/../.."

out=${1:?usage: zygote-artifact.sh <out> <owner>}
owner=${2:?usage: zygote-artifact.sh <out> <owner>}

rootfs=$(mktemp -d)
trap 'rm -rf "$rootfs"' EXIT
hack/gvisor/rootfs.sh "$rootfs" >/dev/null

mkdir -p "$out"
CGO_ENABLED=0 go build -trimpath -o "$out/zygotectl" ./cmd/zygotectl
"$out/zygotectl" build -zygote "$rootfs/bin/refzygote" -args "--heap-mb 32" -skip-images -out "$out/artifact" >/dev/null
chown -R "$owner" "$out"
