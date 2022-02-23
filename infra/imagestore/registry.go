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

package imagestore

import (
	"context"
	"fmt"

	imgcopy "github.com/containers/image/v5/copy"
	"github.com/containers/image/v5/docker/reference"
	"github.com/containers/image/v5/manifest"
	"github.com/containers/image/v5/signature"
	"github.com/containers/image/v5/transports/alltransports"
	"github.com/containers/image/v5/types"
	"github.com/hashicorp/go-multierror"

	"github.com/shipwright-io/image/infra/fs"
)

// CleanFn is a function that must be called in order to clean up or free resources in use.
type CleanFn func()

// Registry wraps calls for iteracting with our backend registry. It provides an implementation
// capable of pushing to and pulling from an image registry. To push an image towards the
// registry one needs to call Load, to push it to a local tar file a Save call should be made,
// this strange naming is an attempt to make it similar to the 'docker save/load' commands.
type Registry struct {
	fs       *fs.FS
	regaddr  string
	insecure bool
	auths    []*types.DockerAuthConfig
	polctx   *signature.PolicyContext
}

// NewRegistry creates an entity capable of load objects to or save objects from a backend
// registry. When calling Load we push an image into the registry, when calling Save we pull
// the image from the registry and store into a local tar file (format in disk is of type
// docker-archive, we should migrate this to something else as it does not support manifest
// lists).
func NewRegistry(
	regaddr string,
	auths []*types.DockerAuthConfig,
	insecure bool,
	polctx *signature.PolicyContext,
) *Registry {
	return &Registry{
		fs:       fs.New(),
		regaddr:  regaddr,
		auths:    auths,
		insecure: insecure,
		polctx:   polctx,
	}
}

// Load pushes an image reference into the backend registry. Uses srcctx (types.SystemContext)
// when reading image from srcref, so when copying from one remote registry into our backend
// registry srcctx must contain all needed authentication information. Images are stored in
// backend.registry.io/namespace/name url.
func (i *Registry) Load(
	ctx context.Context,
	srcref types.ImageReference,
	srcctx *types.SystemContext,
	ns string,
	name string,
) (types.ImageReference, error) {
	tostr := fmt.Sprintf("docker://%s/%s/%s", i.regaddr, ns, name)
	toref, err := alltransports.ParseImageName(tostr)
	if err != nil {
		return nil, fmt.Errorf("invalid destination reference: %w", err)
	}

	insecure := types.OptionalBoolFalse
	if i.insecure {
		insecure = types.OptionalBoolTrue
	}

	var errors *multierror.Error
	for _, auth := range i.registryAuths() {
		manblob, err := imgcopy.Image(
			ctx, i.polctx, toref, srcref, &imgcopy.Options{
				ImageListSelection: imgcopy.CopyAllImages,
				SourceCtx:          srcctx,
				DestinationCtx: &types.SystemContext{
					DockerInsecureSkipTLSVerify: insecure,
					DockerAuthConfig:            auth,
				},
			},
		)
		if err != nil {
			errors = multierror.Append(errors, err)
			continue
		}

		dgst, err := manifest.Digest(manblob)
		if err != nil {
			return nil, fmt.Errorf("error calculating manifest digest: %w", err)
		}

		refstr := fmt.Sprintf("docker://%s@%s", toref.DockerReference().Name(), dgst)
		return alltransports.ParseImageName(refstr)
	}

	return nil, fmt.Errorf("unable to load image: %w", errors)
}

// registryAuths returns a list of auths to be used when attempting to connect to the backend
// registry. If no auth was configured for the backend registrythis function returns an slice
// with a "nil" entry that, in containers/image/v5 library, means no auth (anonymous access).
func (i *Registry) registryAuths() []*types.DockerAuthConfig {
	if len(i.auths) == 0 {
		return []*types.DockerAuthConfig{nil}
	}
	return i.auths
}

// Save pulls an image from our backend registry, stores it in a temporary tar file on disk.
// Returns an ImageReference pointing to the local tar file and a function the caller needs to
// call in order to clean up after our mess (properly close tar file and delete it from disk).
// Returned ref points to a 'docker-archive' tar file.
func (i *Registry) Save(
	ctx context.Context, ref types.ImageReference,
) (types.ImageReference, CleanFn, error) {
	domain := reference.Domain(ref.DockerReference())
	if domain != i.regaddr {
		return nil, nil, fmt.Errorf("backend registry doesn't know about this image")
	}

	insecure := types.OptionalBoolFalse
	if i.insecure {
		insecure = types.OptionalBoolTrue
	}

	var errors *multierror.Error
	for _, auth := range i.registryAuths() {
		destref, cleanup, err := i.NewLocalReference()
		if err != nil {
			return nil, nil, fmt.Errorf("error creating temp file: %w", err)
		}

		if _, err := imgcopy.Image(
			ctx, i.polctx, destref, ref, &imgcopy.Options{
				SourceCtx: &types.SystemContext{
					DockerInsecureSkipTLSVerify: insecure,
					DockerAuthConfig:            auth,
				},
			},
		); err != nil {
			cleanup()
			errors = multierror.Append(errors, err)
			continue
		}
		return destref, cleanup, nil
	}

	return nil, nil, fmt.Errorf("unable to save image: %w", errors)
}

// NewLocalReference returns an image reference pointing to a local tar file. Also returns a
// clean up function that must be called to free resources.
func (i *Registry) NewLocalReference() (types.ImageReference, CleanFn, error) {
	tfile, cleanup, err := i.fs.TempFile()
	if err != nil {
		return nil, nil, fmt.Errorf("error creating temp file: %w", err)
	}
	fpath := fmt.Sprintf("docker-archive:%s", tfile.Name())

	ref, err := alltransports.ParseImageName(fpath)
	if err != nil {
		cleanup()
		return nil, nil, fmt.Errorf("error creating new local ref: %w", err)
	}
	return ref, cleanup, nil
}
