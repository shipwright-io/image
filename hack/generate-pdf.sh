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

# This scripts generate a pdf out of the README.md file. Useful when we want
# to make the documentation available in a per release basis. Requires pandoc
# and texlive-extra-utils installed in the system.

OUTPUT_DOC=output/doc
rm -rf $OUTPUT_DOC && mkdir -p $OUTPUT_DOC
cat README.md | pandoc \
	-fmarkdown-implicit_figures \
	-V geometry:margin=1in \
	-o $OUTPUT_DOC/README.pdf
