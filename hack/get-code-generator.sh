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

# This script clones the code-generator repository inside the output directory.
# code-generator is used to generate kubernetes clients, informers, listers, etc
# Requires go and git binaries.

GENERATOR_VERSION=v0.22.0
OUTPUT_DIR=output/code-generator

rm -rf $OUTPUT_DIR
git clone --depth=1 \
	--branch $GENERATOR_VERSION \
	https://github.com/kubernetes/code-generator.git \
	$OUTPUT_DIR

cd $OUTPUT_DIR && go mod vendor
