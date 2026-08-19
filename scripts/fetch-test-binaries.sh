#!/usr/bin/env bash

set -e

k8s_version=1.29.x
tmp_root=./_out
kb_root_dir=$tmp_root/kubebuilder

NO_COLOR=${NO_COLOR:-""}
if [ -z "$NO_COLOR" ]; then
  header=$'\e[1;33m'
  reset=$'\e[0m'
else
  header=''
  reset=''
fi

function header_text {
  echo "$header$*$reset"
}

header_text "fetching kubebuilder tools via setup-envtest"

if ! command -v setup-envtest &> /dev/null; then
  header_text "installing setup-envtest..."
  go install sigs.k8s.io/controller-runtime/tools/setup-envtest@${SETUP_ENVTEST_VERSION:-latest}
fi

mkdir -p "$kb_root_dir/bin"

ENVTEST_DIR=$(setup-envtest use "$k8s_version" --bin-dir "$kb_root_dir" -p path)
header_text "envtest binaries installed at: $ENVTEST_DIR"

# Symlink binaries to the expected location for the Makefile
for bin in etcd kube-apiserver kubectl; do
  if [ -f "$ENVTEST_DIR/$bin" ]; then
    ln -sf "$(cd "$ENVTEST_DIR" && pwd)/$bin" "$kb_root_dir/bin/$bin"
  fi
done

header_text "kubebuilder tools (etcd, kubectl, kube-apiserver) available at: $kb_root_dir/bin/"
