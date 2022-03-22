// Copyright 2020 The Shipwright Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package controllers

import (
	"context"

	"k8s.io/client-go/tools/cache"

	imgv1b1 "github.com/shipwright-io/image/infra/images/v1beta1"
	imginformer "github.com/shipwright-io/image/infra/images/v1beta1/gen/informers/externalversions"
)

type imgimportsvc struct {
	imginf imginformer.SharedInformerFactory
}

func (t *imgimportsvc) Sync(context.Context, *imgv1b1.ImageImport) error {
	return nil
}

func (t *imgimportsvc) Get(context.Context, string, string) (*imgv1b1.ImageImport, error) {
	return nil, nil
}

func (t *imgimportsvc) AddEventHandler(handler cache.ResourceEventHandler) {
	t.imginf.Shipwright().V1beta1().ImageImports().Informer().AddEventHandler(handler)
}
