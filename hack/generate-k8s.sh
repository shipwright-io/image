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

# This script generates the kubernetes client (lister, informers, etc). Input
# files are read from the infra/image/v1beta1 directory. This script uses the
# binary downloaded by get-code-generator.sh script.

PROJECT=github.com/shipwright-io/image
GEN_BINARY_DIR=output/code-generator
GEN_OUTPUT=/tmp/$PROJECT/infra/images

rm -rf $GEN_OUTPUT
$GEN_BINARY_DIR/generate-groups.sh all \
	$PROJECT/infra/images/v1beta1/gen \
	$PROJECT \
	infra/images:v1beta1 \
	--go-header-file=$GEN_BINARY_DIR/hack/boilerplate.go.txt \
	--output-base=/tmp

rm -rf infra/images/v1beta1/gen
mv $GEN_OUTPUT/v1beta1/* infra/images/v1beta1/
