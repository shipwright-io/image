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

package services

import (
	"context"
	"fmt"
	"os"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/informers"
	corelister "k8s.io/client-go/listers/core/v1"
	"k8s.io/kubernetes/pkg/credentialprovider"
	"k8s.io/kubernetes/pkg/credentialprovider/secrets"

	"github.com/containers/image/v5/signature"
	"github.com/containers/image/v5/transports/alltransports"
	"github.com/containers/image/v5/types"

	"github.com/shipwright-io/image/infra/imagestore"
)

// MirrorRegistryConfig holds the needed data that allows imgctrl to contact the mirror registry.
type MirrorRegistryConfig struct {
	Address  string
	Insecure bool
}

// SysContext groups tasks related to system context/configuration, deal with things such as
// configured docker authentications or unqualified registries configs.
type SysContext struct {
	sclister              corelister.SecretLister
	cmlister              corelister.ConfigMapLister
	unqualifiedRegistries []string
}

// NewSysContext returns a new SysContext helper.
func NewSysContext(corinf informers.SharedInformerFactory) *SysContext {
	var sclister corelister.SecretLister
	var cmlister corelister.ConfigMapLister
	if corinf != nil {
		sclister = corinf.Core().V1().Secrets().Lister()
		cmlister = corinf.Core().V1().ConfigMaps().Lister()
	}

	return &SysContext{
		sclister:              sclister,
		cmlister:              cmlister,
		unqualifiedRegistries: []string{"docker.io"},
	}
}

// UnqualifiedRegistries returns the list of unqualified registries configured on the system.
// XXX this is a place holder as we most likely gonna need to read this from a configuration
// somewhere.
func (s *SysContext) UnqualifiedRegistries(ctx context.Context) ([]string, error) {
	return s.unqualifiedRegistries, nil
}

// ParseShipwrightMirrorRegistryConfig parses a secret called "mirror-registry-config" in the pod
// namespace. This secret holds information on how to connect to the mirror registry.
func (s *SysContext) ParseShipwrightMirrorRegistryConfig() (MirrorRegistryConfig, error) {
	var zero MirrorRegistryConfig

	namespace := os.Getenv("POD_NAMESPACE")
	if len(namespace) == 0 {
		return zero, fmt.Errorf("unbound POD_NAMESPACE variable")
	}

	sct, err := s.sclister.Secrets(namespace).Get("mirror-registry-config")
	if err != nil {
		return zero, fmt.Errorf("unable to read registry config: %w", err)
	}
	if len(sct.Data) == 0 {
		return zero, fmt.Errorf("registry config is empty")
	}

	return MirrorRegistryConfig{
		Address:  string(sct.Data["address"]),
		Insecure: string(sct.Data["insecure"]) == "true",
	}, nil
}

// SystemContextsFor builds a series of types.SystemContexts, all of them using one of the auth
// credentials present in the namespace. The last entry is always a nil SystemContext, this last
// entry means "no auth". Insecure indicate if the returned SystemContexts tolerate invalid TLS
// certificates.
func (s *SysContext) SystemContextsFor(
	ctx context.Context,
	imgref types.ImageReference,
	namespace string,
	insecure bool,
) ([]*types.SystemContext, error) {
	auths, err := s.authsFor(ctx, imgref, namespace)
	if err != nil {
		return nil, fmt.Errorf("error reading auths: %w", err)
	}

	optinsecure := types.OptionalBoolFalse
	if insecure {
		optinsecure = types.OptionalBoolTrue
	}

	ctxs := make([]*types.SystemContext, len(auths))
	for i, auth := range auths {
		ctxs[i] = &types.SystemContext{
			DockerInsecureSkipTLSVerify: optinsecure,
			DockerAuthConfig:            auth,
		}
	}

	// here we append a SystemContext without authentications set, we want to allow imports
	// without using authentication. This entry will be nil if we want to use the system
	// defaults.
	var noauth *types.SystemContext
	if insecure {
		noauth = &types.SystemContext{
			DockerInsecureSkipTLSVerify: optinsecure,
		}
	}

	ctxs = append(ctxs, noauth)
	return ctxs, nil
}

// authsFor return configured authentications for the registry hosting the image reference.
// Namespace is the namespace from where read docker authentications.
func (s *SysContext) authsFor(
	ctx context.Context, imgref types.ImageReference, namespace string,
) ([]*types.DockerAuthConfig, error) {
	secsref, err := s.sclister.Secrets(namespace).List(labels.Everything())
	if err != nil {
		return nil, fmt.Errorf("fail to list secrets: %w", err)
	}

	var secs []v1.Secret
	for _, sct := range secsref {
		ref := sct.DeepCopy()
		secs = append(secs, *ref)
	}

	keyring, err := secrets.MakeDockerKeyring(secs, credentialprovider.NewDockerKeyring())
	if err != nil {
		return nil, fmt.Errorf("unable to build keyring: %w", err)
	}

	imgstr := imgref.DockerReference().String()
	auths, _ := keyring.Lookup(imgstr)

	var dockerAuths []*types.DockerAuthConfig
	for _, auth := range auths {
		dockerAuths = append(
			dockerAuths,
			&types.DockerAuthConfig{
				Username:      auth.Username,
				Password:      auth.Password,
				IdentityToken: auth.IdentityToken,
			},
		)
	}
	return dockerAuths, nil
}

// DefaultPolicyContext returns the default policy context. XXX this should be reviewed.
func (s *SysContext) DefaultPolicyContext() (*signature.PolicyContext, error) {
	pol := &signature.Policy{
		Default: signature.PolicyRequirements{
			signature.NewPRInsecureAcceptAnything(),
		},
	}
	return signature.NewPolicyContext(pol)
}

// GetRegistryStore creates an instance of a Registry store entity configured to use our mirror
// registry as underlying storage. This store may vary from one namespace to another as this
// function sets up authentications (and these are read from the namespace).
func (s *SysContext) GetRegistryStore(
	ctx context.Context, namespace, name string,
) (*imagestore.Registry, error) {
	defpol, err := s.DefaultPolicyContext()
	if err != nil {
		return nil, fmt.Errorf("error reading default policy: %w", err)
	}

	mcfg, err := s.ParseShipwrightMirrorRegistryConfig()
	if err != nil {
		return nil, fmt.Errorf("unable to acccess mirror: %w", err)
	}

	// we create a "fake" image reference pointing to the mirror registry and then
	// attempt to read authentication for the reference from the target namespace.
	dststr := fmt.Sprintf("docker://%s/%s/%s", mcfg.Address, namespace, name)
	imgref, err := alltransports.ParseImageName(dststr)
	if err != nil {
		return nil, fmt.Errorf("unable to parse reference to mirror: %w", err)
	}

	auths, err := s.authsFor(ctx, imgref, namespace)
	if err != nil {
		return nil, fmt.Errorf("unable to read mirror config: %w", err)
	}

	return imagestore.NewRegistry(mcfg.Address, auths, mcfg.Insecure, defpol), nil
}

// RegistriesToSearch returns a list of registries to be used when looking for an image. It is
// either the provided domain or a list of unqualified domains configured globally and returned
// by UnqualifiedRegistries(). This function is used when trying to understand what an user means
// when she/he simply asks to import an image called "centos:latest" for instance, in what
// registries do we need to look for this image? This is the place where we can implement a mirror
// search.
func (s *SysContext) RegistriesToSearch(ctx context.Context, domain string) ([]string, error) {
	if domain != "" {
		return []string{domain}, nil
	}
	registries, err := s.UnqualifiedRegistries(ctx)
	if err != nil {
		return nil, err
	}

	if len(registries) == 0 {
		return nil, fmt.Errorf("no unqualified registries found")
	}
	return registries, nil
}
