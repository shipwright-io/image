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
	"encoding/json"
	"fmt"
	"os"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/informers"
	corelister "k8s.io/client-go/listers/core/v1"
	"k8s.io/klog/v2"

	"github.com/containers/image/v5/docker/reference"
	"github.com/containers/image/v5/signature"
	"github.com/containers/image/v5/types"

	"github.com/shipwright-io/image/infra/imagestore"
)

// We use dockerAuthConfig to unmarshal a default docker configuration present on secrets of
// type SecretTypeDockerConfigJson. XXX doesn't containers/image export a similar structure?
// Or maybe even a function to parse a docker configuration file?
type dockerAuthConfig struct {
	Auths map[string]types.DockerAuthConfig
}

// MirrorRegistryConfig holds the needed data that allows imgctrl to contact the mirror registry.
type MirrorRegistryConfig struct {
	Address    string
	Username   string
	Password   string
	Repository string
	Token      string
	Insecure   bool
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

// MirrorConfig returns the mirror configuration as read from provided namespace or from the
// operator's namespace.
func (s *SysContext) MirrorConfig(namespace string) (*MirrorRegistryConfig, error) {
	cfg, err := s.parseMirrorConfig(namespace)
	if err != nil && !errors.IsNotFound(err) {
		return nil, fmt.Errorf("unable to load local mirror config: %w", err)
	} else if err == nil {
		return cfg, nil
	}

	namespace = os.Getenv("POD_NAMESPACE")
	if len(namespace) == 0 {
		return nil, fmt.Errorf("unbound POD_NAMESPACE variable")
	}

	cfg, err = s.parseMirrorConfig(namespace)
	if err != nil {
		return nil, fmt.Errorf("unable to load global mirror config: %w", err)
	}
	return cfg, nil
}

// parseMirrorConfig attempts to parse the mirror configuration present in the provided
// namespace. Reads the secret and returns a populated MirrorRegistryConfig struct.
func (s *SysContext) parseMirrorConfig(namespace string) (*MirrorRegistryConfig, error) {
	sct, err := s.sclister.Secrets(namespace).Get("mirror-registry-config")
	if err != nil {
		return nil, err
	}

	if len(sct.Data) == 0 {
		return nil, fmt.Errorf("empty mirror registry config found")
	}

	return &MirrorRegistryConfig{
		Address:    string(sct.Data["address"]),
		Username:   string(sct.Data["username"]),
		Password:   string(sct.Data["password"]),
		Repository: string(sct.Data["repository"]),
		Token:      string(sct.Data["token"]),
		Insecure:   string(sct.Data["insecure"]) == "true",
	}, nil
}

// MirrorRegistryContext returns the context to be used when talking to the the registry used
// for mirroring images.
func (s *SysContext) MirrorRegistryContext(
	ctx context.Context, namespace string,
) (*types.SystemContext, error) {
	cfg, err := s.MirrorConfig(namespace)
	if err != nil {
		klog.Infof("unable to read imgctrl mirror registry config: %s", err)
		return nil, fmt.Errorf("unable to read imgctrl mirror registry config: %w", err)
	}

	insecure := types.OptionalBoolFalse
	if cfg.Insecure {
		insecure = types.OptionalBoolTrue
	}

	return &types.SystemContext{
		DockerInsecureSkipTLSVerify: insecure,
		DockerAuthConfig: &types.DockerAuthConfig{
			Username:      cfg.Username,
			Password:      cfg.Password,
			IdentityToken: cfg.Token,
		},
	}, nil
}

// SystemContextsFor builds a series of types.SystemContexts, all of them using one of the auth
// credentials present in the namespace. The last entry is always a nil SystemContext, this last
// entry means "no auth". Insecure indicate if the returned SystemContexts tolerate invalid TLS
// certificates.
func (s *SysContext) SystemContextsFor(
	ctx context.Context, imgref types.ImageReference, namespace string, insecure bool,
) ([]*types.SystemContext, error) {
	// if imgref points to an image hosted in our mirror registry we return a SystemContext
	// using default user and pass (the ones user has configured imgctrl with). XXX i am not
	// sure yet this is a good idea permission wide.
	domain := reference.Domain(imgref.DockerReference())

	cfg, err := s.MirrorConfig(namespace)
	if err != nil {
		klog.Infof("no mirror registry configured, moving on")
	} else if cfg.Address == domain {
		mirrorctx, err := s.MirrorRegistryContext(ctx, namespace)
		if err != nil {
			return nil, fmt.Errorf("error reading mirror config: %w", err)
		}
		return []*types.SystemContext{mirrorctx}, nil
	}

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
	secrets, err := s.sclister.Secrets(namespace).List(labels.Everything())
	if err != nil {
		return nil, fmt.Errorf("fail to list secrets: %w", err)
	}

	domain := reference.Domain(imgref.DockerReference())
	if domain == "" {
		return nil, nil
	}

	var dockerAuths []*types.DockerAuthConfig
	for _, sec := range secrets {
		if sec.Type != corev1.SecretTypeDockerConfigJson {
			continue
		}

		secdata, ok := sec.Data[corev1.DockerConfigJsonKey]
		if !ok {
			continue
		}

		var cfg dockerAuthConfig
		if err := json.Unmarshal(secdata, &cfg); err != nil {
			klog.Infof("ignoring secret %s/%s: %s", sec.Namespace, sec.Name, err)
			continue
		}

		sec, ok := cfg.Auths[domain]
		if !ok {
			continue
		}

		dockerAuths = append(dockerAuths, &sec)
	}
	return dockerAuths, nil
}

// RegistryStore creates an instance of an Registry store entity configured to use our mirror
// registry as underlying storage.
func (s *SysContext) RegistryStore(ctx context.Context, ns string) (*imagestore.Registry, error) {
	pol := &signature.Policy{
		Default: signature.PolicyRequirements{
			signature.NewPRInsecureAcceptAnything(),
		},
	}

	defpol, err := signature.NewPolicyContext(pol)
	if err != nil {
		return nil, fmt.Errorf("error reading default policy: %w", err)
	}

	mcfg, err := s.MirrorConfig(ns)
	if err != nil {
		return nil, fmt.Errorf("unable to acccess mirror: %w", err)
	}

	sysctx, err := s.MirrorRegistryContext(ctx, ns)
	if err != nil {
		return nil, fmt.Errorf("unable to read mirror config: %w", err)
	}

	return imagestore.NewRegistry(mcfg.Address, mcfg.Repository, sysctx, defpol), nil
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
