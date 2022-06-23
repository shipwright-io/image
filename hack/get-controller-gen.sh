#!/usr/bin/env bash
set -eu -o pipefail

# Copyright 2020 The Shipwright Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#       http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# This script downloads a specific version of the controller-gen utility. This
# binary is used when generating CRD yamls out of go types.

VERSION=v0.9.0
OUTPUT_DIR=${PWD}/output/controller-gen

rm -rf $OUTPUT_DIR && mkdir -p $OUTPUT_DIR
GOBIN=$OUTPUT_DIR GOFLAGS="" go install sigs.k8s.io/controller-tools/cmd/controller-gen@$VERSION

