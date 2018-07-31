// Copyright 2016-2018, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package providers

import (
	"fmt"
	"sync"

	"github.com/blang/semver"
	uuid "github.com/satori/go.uuid"

	"github.com/pulumi/pulumi/pkg/resource"
	"github.com/pulumi/pulumi/pkg/resource/config"
	"github.com/pulumi/pulumi/pkg/resource/plugin"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/util/contract"
	"github.com/pulumi/pulumi/pkg/util/logging"
)

func getProviderVersion(inputs resource.PropertyMap) (*semver.Version, error) {
	versionProp, ok := properties["version"]
	if !ok {
		return nil, nil
	}

	if !versionProp.IsString() {
		return errors.New("'version' must be a string")
	}

	sv, err := semver.ParseTolerant(versionProp.StringValue())
	if err != nil {
		return errors.Errorf("could not parse provider version: %v", err)
	}
	return sv, nil
}


type Registry struct {
	host plugin.Host
	isPreview bool
	providers map[Reference]plugin.Provider
	m sync.RWMutex
}

var _ plugin.Provider = (*Registry)(nil)

func NewRegistry(host plugin.Host, prev []*resource.State, isPreview bool) (*Registry, error) {
	r := &Registry{
		host: host,
		isPreview: isPreview,
		providers: make(map[Reference]plugin.Provider),
	}

	for _, res := range prev {
		urn := res.URN
		if !IsProviderType(urn.Type()) {
			continue
		}

		// Ensure that this provider has a known ID.
		if res.ID == "" || res.ID == UnknownID {
			return nil, errors.Errorf("provider '%v' has an unknown ID", urn)
		}

		// Parse the provider version, then load, configure, and register the provider.
		version, err := getProviderVersion(res.Inputs)
		if err != nil {
			return nil, errors.Errorf("could not parse version for provider '%v': %v", urn, err)
		}
		provider, err := host.Provider(getProviderPackage(urn.Type()), version)
		if err != nil {
			return nil, errors.Errorf("could not load provider '%v': %v", urn, err)
		}
		if err := provider.Configure(res.Inputs); err != nil {
			closeErr = host.CloseProvider(provider)
			contract.IgnoreError(closeErr)
			return nil, errors.Errof("could not configure provider '%v': %v", urn, err)
		}
		r.providers[mustNewReference(urn, id)] = provider
	}

	return r, nil
}

func (r *registry) GetProvider(ref Reference) (plugin.Provider, bool) {
	r.m.RLock()
	defer r.m.RUnlock()

	provider, ok := r.providers[ref]
	return provider, ok
}

func (r *registry) setProvider(ref Reference, provider plugin.Provider) {
	r.m.Lock()
	defer r.m.Unlock()

	r.providers[ref] = provider
}

func (r *registry) deleteProvider(ref Reference) (plugin.Provider, bool) {
	r.m.Lock()
	defer r.m.Unlock()

	provider, ok := r.providers[ref]
	if !ok {
		return nil, false
	}
	delete(r.providers, ref)
	return provider, true
}

func (r *registry) Close() error {
	return nil
}

func (r *registry) Pkg() tokens.Package {
	return "pulumi"
}

func (r *registry) Configure(props map[config.Key]string) error {
	contract.Fail()
	return errors.New("the metaProvider is not configurable")
}

func (r *registry) Check(urn resource.URN, olds, news resource.PropertyMap,
	allowUnknowns bool) (resource.PropertyMap, []plugin.CheckFailure, error) {

	contract.Require(IsProviderType(urn.Type()), "urn")

	// Parse the version from the provider properties and load the provider.
	version, err := getProviderVersion(news)
	if err != nil {
		return nil, []plugin.CheckFailure{Property: "version", Reason: err.String()}, nil
	}
	provider, err := r.host.Provider(getProviderPackage(urn.Type()), version)
	if err != nil {
		return nil, nil, err
	}

	// Check the provider's config. If the check fails, unload the provider.
	inputs, failures, err := provider.CheckConfig(olds, news)
	if len(failures) != 0 || err != nil {
		closeErr := r.host.CloseProvider(provider)
		contract.IgnoreError(closeErr)
		return nil, failures, err
	}

	// If we are running a preview, configure the provider now. If we are not running a preview, we will configure the
	// provider when it is created or updated.
	if r.isPreview {
		if err := provider.Configure(inputs); err != nil {
			closeErr := r.host.CloseProvider(provider)
			contract.IgnoreError(closeErr)
			return nil, nil, err
		}
	}

	// Create a provider reference using the URN and the unknown ID and register the provider.
	r.setProvider(mustNewReference(urn, UnknownID), provider)

	return inputs, nil, nil
}

func (r *registry) Diff(urn resource.URN, id resource.ID, olds, news resource.PropertyMap,
	allowUnknowns bool) (plugin.DiffResult, error) {

	contract.Require(id != "", "id")

	// Create a reference using the URN and the unknown ID and fetch the provider.
	provider, ok = r.GetProvider(mustNewReference(urn, UnknownID))
	contract.Assertf(ok, "'Check' must be called before 'Diff'")

	// Diff the properties.
	diff, err := provider.DiffConfig(olds, news)
	if err != nil {
		return plugin.DiffResult{Changes: plugin.DiffUnknown}, err
	}

	// If the diff requires replacement, unload the provider: the engine will reload it during its replacememnt Check.
	// If the diff does not require replacement and we are running a preview, register it under its current ID.
	if len(diff.ReplaceKeys) != 0 {
		closeErr := r.host.CloseProvider(provider)
		contract.IgnoreError(closeErr)
	} else if r.isPreview {
		r.setProvider(mustNewReference(urn, id), provider)
	}

	return diff, nil
}

func (r *registry) Create(urn resource.URN,
	news resource.PropertyMap) (resource.ID, resource.PropertyMap, resource.Status, error) {

	contract.Assert(!isPreview)

	// Fetch the unconfigured provider, configure it, and register it under a new ID.
	provider, ok := r.GetProvider(mustNewReference(urn, UnknownID))
	contract.Assertf(ok, "'Check' must be called before 'Create'")

	if err := provider.Configure(news); err != nil {
		return "", nil, resource.StatusOK, err
	}

	id := uuid.NewV4().String()
	contract.Assert(id != UnknownID)

	r.setProvider(mustNewReference(urn, id), provider)
	return id, resource.PropertyMap{}, resource.StatusOK, nil
}

func (r *registry) Read(urn resource.URN, id resource.ID,
	props resource.PropertyMap) (resource.PropertyMap, error) {
	contract.Fail()
	return nil, errors.New("providers may not be read")
}

func (r *registry) Update(urn resource.URN, id resource.ID, olds,
	news resource.PropertyMap) (resource.PropertyMap, resource.Status, error) {

	// Fetch the unconfigured provider and configure it.
	provider, ok := r.GetProvider(mustNewReference(urn, id))
	contract.Assertf(ok, "'Check' and 'Diff' must be called before 'Update'")

	if err := provider.Configure(news); err != nil {
		return nil, resource.StatusUnknown, err
	}

	return resource.PropertyMap{}, resource.StatusOK, nil
}

func (r *registry) Delete(urn resource.URN, id resource.ID, props resource.PropertyMap) (resource.Status, error) {
	ref := mustNewReference(urn, id)
	provider, ok := r.deleteProvider(ref)
	if !ok {
		return resource.StatusUnknown, errors.Errorf("unknown provider '%v'", ref)
	}
	closeErr := r.host.CloseProvider(provider)
	contract.IgnoreError(closeErr)
	return resource.StatusOK, nil
}

func (r *registry) Invoke(tok tokens.ModuleMember,
	args resource.PropertyMap) (resource.PropertyMap, []plugin.CheckFailure, error) {
	contract.Fail()
	return nil, nil, errors.New("the metaProvider is not invokeable")
}

func (r *registry) GetPluginInfo() (workspace.PluginInfo, error) {
	// return an error: this should not be called for the metaProvider
	contract.Fail()
	return workspace.PluginInfo{}, errors.New("the metaProvider does not report plugin info")
}

func (r *registry) SignalCancellation() error {
	// TODO: this should probably cancel any outstanding load requests and return
	return nil
}
