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

# This scripts downloads a specific kuttl version and stores it under the
# output directory (output/kuttl).

KUTTL_VERSION=0.11.1
KUTTL_DIR=output/kuttl
KUTTL_REPO=https://github.com/kudobuilder/kuttl
rm -rf $KUTTL_DIR && mkdir -p $KUTTL_DIR

curl -o $KUTTL_DIR/kuttl -L \
	$KUTTL_REPO/releases/download/v$KUTTL_VERSION/kubectl-kuttl_$KUTTL_VERSION\_linux_x86_64

chmod 755 $KUTTL_DIR/kuttl
