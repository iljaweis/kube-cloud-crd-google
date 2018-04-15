#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

if [ -d "${GOPATH}/src/k8s.io/code-generator" ]; then
  CODEGEN_PKG=${GOPATH}/src/k8s.io/code-generator
fi

SCRIPT_ROOT=`pwd`/$(dirname ${BASH_SOURCE})/..
CODEGEN_PKG=${CODEGEN_PKG:-$(cd ${SCRIPT_ROOT}; ls -d -1 ./vendor/k8s.io/code-generator 2>/dev/null || echo ../code-generator)}

( cd $CODEGEN_PKG

./generate-groups.sh "deepcopy,client,informer,lister" \
  github.com/iljaweis/kube-cloud-crd-google/pkg/client github.com/iljaweis/kube-cloud-crd-google/pkg/apis \
  google.cloudcrd.weisnix.org:v1 \
  --go-header-file ${SCRIPT_ROOT}/hack/custom-boilerplate.go.txt

)
