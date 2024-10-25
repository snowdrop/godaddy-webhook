#!/usr/bin/env bash

set -e

k8s_version=1.29.1
goarch=$(go env GOARCH)
goos="unknown"

if [[ "$OSTYPE" == "linux-gnu" ]]; then
  goos="linux"
elif [[ "$OSTYPE" == "darwin"* ]]; then
  goos="darwin"
fi

if [[ "$goos" == "unknown" ]]; then
  echo "OS '$OSTYPE' not supported. Aborting." >&2
  exit 1
fi

tmp_root=./_out
kb_root_dir=$tmp_root/kubebuilder

# Turn colors in this script off by setting the NO_COLOR variable in your
# environment to any value:
#
# $ NO_COLOR=1 test.sh
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

# fetch k8s API gen tools and make it available under kb_root_dir/bin.
function fetch_kb_tools {
  header_text "fetching tools"
  mkdir -p $tmp_root
  kb_tools_archive_name="kubebuilder-tools-$k8s_version-$goos-$goarch.tar.gz"
  kb_tools_download_url="https://storage.googleapis.com/kubebuilder-tools/$kb_tools_archive_name"
  echo "URL: $kb_tools_download_url"

  kb_tools_archive_path="$tmp_root/$kb_tools_archive_name"
  if [ ! -f $kb_tools_archive_path ]; then
    curl -sL ${kb_tools_download_url} -o "$kb_tools_archive_path"
  fi
  tar -zvxf "$kb_tools_archive_path" -C "$tmp_root/"
}

fetch_kb_tools

header_text "kubebuilder v$k8s_version tools (etcd, kubectl, kube-apiserver) used to perform local tests. It has been installed: $tmp_root/kubebuilder/bin/"
exit 0
