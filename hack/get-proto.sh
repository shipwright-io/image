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

# This script installs a pinned version of protoc and protoc-gen-go binaries
# under output/protoc directory.

PROTO_VER=3.15.8
GEN_GO_VER=v1
PROTO_OUT_DIR=${PWD}/output/protoc
PB_REL="https://github.com/protocolbuffers/protobuf/releases"

# Downloads protoc binary and stores it under output/protoc directory.
rm -rf $PROTO_OUT_DIR && mkdir -p $PROTO_OUT_DIR
TMP_DIR=$(mktemp -d -t proto-XXXX)
curl -o $TMP_DIR/protoc.zip -L $PB_REL/download/v$PROTO_VER/protoc-$PROTO_VER-linux-x86_64.zip
unzip $TMP_DIR/protoc.zip -d $TMP_DIR
mv $TMP_DIR/bin/protoc $PROTO_OUT_DIR/
rm -rf $TMP_DIR

# uses go to install protoc-gen-go binary under output/protoc directory.
GOFLAGS="" GOBIN=$PROTO_OUT_DIR \
	go install google.golang.org/protobuf/cmd/protoc-gen-go@$GEN_GO_VER
