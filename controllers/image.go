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
	"fmt"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

	imgv1b1 "github.com/shipwright-io/image/infra/images/v1beta1"
)

// ImageSyncer abstraction exists to make testing easier. You most likely wanna see Image struct
// under services/image.go for a concrete implementation of this.
type ImageSyncer interface {
	Sync(context.Context, *imgv1b1.Image) error
	Get(context.Context, string, string) (*imgv1b1.Image, error)
	AddEventHandler(cache.ResourceEventHandler)
}

// Image controller handles events related to Images. It starts and receives events from the
// informer, calling appropriate functions on our concrete services layer implementation.
type Image struct {
	queue  workqueue.RateLimitingInterface
	imgsvc ImageSyncer
	appctx context.Context
	tokens chan bool
}

// NewImage returns a new controller for Images. This controller runs image imports in parallel,
// at a given time we can have at max "tokens" distinct images being processed (hardcoded to 10).
func NewImage(imgsvc ImageSyncer, impsvc ImageImportSyncer) *Image {
	ratelimit := workqueue.NewItemExponentialFailureRateLimiter(time.Second, time.Minute)
	ctrl := &Image{
		queue:  workqueue.NewRateLimitingQueue(ratelimit),
		imgsvc: imgsvc,
		tokens: make(chan bool, 10),
	}
	imgsvc.AddEventHandler(ctrl.handlers())
	impsvc.AddEventHandler(ctrl.handlers())
	return ctrl
}

// Name returns a name identifier for this controller.
func (t *Image) Name() string {
	return "image"
}

// RequiresLeaderElection returns if this controller requires or not a leader lease to run.
func (t *Image) RequiresLeaderElection() bool {
	return true
}

// enqueueEvent generates a key using "namespace/name" for the event received and then enqueues
// it to be processed. "namespace" and "name" refer to the image that generated the event. If the
// received event is for an ImageImport object we extract the "target image" information from it
// and enqueue an event for the Image instead.
func (t *Image) enqueueEvent(obj interface{}) {
	// if the event is for an Image object we extract the meta namespace key and enqueue it
	// without further processing.
	if _, ok := obj.(*imgv1b1.Image); ok {
		key, err := cache.MetaNamespaceKeyFunc(obj)
		if err != nil {
			klog.Errorf("fail to enqueue event: %v : %s", obj, err)
			return
		}
		t.queue.AddRateLimited(key)
		return
	}

	// from this point on we expect the event to be related to an ImageImport object. If it
	// is not we log an error but if it is then we extract the target image information and
	// enqueue an event for it instead.
	ii, ok := obj.(*imgv1b1.ImageImport)
	if !ok {
		klog.Errorf("received event for an unkonwn object type: %+v", obj)
		return
	}

	key := fmt.Sprintf("%s/%s", ii.Namespace, ii.Spec.Image)
	t.queue.AddRateLimited(key)
}

// handlers return a event handler that will be called by the informer whenever an event occurs.
// This handler basically enqueues everything in our work queue using enqueueEvent.
func (t *Image) handlers() cache.ResourceEventHandler {
	return cache.ResourceEventHandlerFuncs{
		AddFunc: func(o interface{}) {
			t.enqueueEvent(o)
		},
		UpdateFunc: func(o, n interface{}) {
			t.enqueueEvent(o)
		},
		DeleteFunc: func(o interface{}) {
			t.enqueueEvent(o)
		},
	}
}

// eventProcessor reads our events calling syncImage for all of them. Uses t.tokens to control
// how many images are processed in parallel.
func (t *Image) eventProcessor(wg *sync.WaitGroup) {
	var running sync.WaitGroup
	defer wg.Done()
	for {
		evt, end := t.queue.Get()
		if end {
			klog.Info("queue closed, awaiting for running workers")
			running.Wait()
			klog.Info("all running workers finished")
			return
		}

		t.tokens <- true
		running.Add(1)
		go func() {
			defer func() {
				<-t.tokens
				running.Done()
			}()

			namespace, name, err := cache.SplitMetaNamespaceKey(evt.(string))
			if err != nil {
				klog.Errorf("invalid event received %s: %s", evt, err)
				t.queue.Done(evt)
				return
			}

			klog.Infof("received event for image : %s", evt)
			if err := t.syncImage(namespace, name); err != nil {
				klog.Errorf("error processing image %s: %v", evt, err)
				t.queue.Done(evt)
				t.queue.AddRateLimited(evt)
				return
			}

			klog.Infof("event for image %s processed", evt)
			t.queue.Done(evt)
			t.queue.Forget(evt)
		}()
	}
}

// syncImage process an event for an Image. A max of one minute is allowed per image sync.
func (t *Image) syncImage(namespace, name string) error {
	ctx, cancel := context.WithTimeout(t.appctx, time.Minute)
	defer cancel()

	it, err := t.imgsvc.Get(ctx, namespace, name)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return err
	}
	return t.imgsvc.Sync(ctx, it)
}

// Start starts the controller's event loop.
func (t *Image) Start(ctx context.Context) error {
	// appctx is the 'keep going' context, if it is cancelled
	// everything we might be doing should stop.
	t.appctx = ctx

	var wg sync.WaitGroup
	wg.Add(1)
	go t.eventProcessor(&wg)

	// wait until it is time to die.
	<-t.appctx.Done()

	t.queue.ShutDown()
	wg.Wait()
	return nil
}
