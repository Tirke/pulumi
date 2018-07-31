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

	"github.com/pulumi/pulumi/pkg/resource"
	"github.com/pulumi/pulumi/pkg/resource/config"
	"github.com/pulumi/pulumi/pkg/resource/plugin"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/util/contract"
	"github.com/pulumi/pulumi/pkg/util/logging"
)

// ProviderRegistry allows access to providers at runtime.
type Registry interface {
	// RegisterProvider loads and registers a provider for the given URN.
	RegisterProvider(urn resource.URN, properties resource.PropertyMap,
		allowUnknowns bool) (Reference, plugin.Provider, []plugin.CheckFailure, error)
	// GetProvider returns the provider plugin for the given reference.
	GetProvider(ref Reference) (plugin.Provider, bool)
}

func NewRegistry(host plugin.Host) Registry {
	return &registry{
		host: host,
		providers: make(map[Reference]plugin.Provider),
	}
}

// registry is a concrete implementation of the provider registry.
type registry struct {
	host plugin.Host
	providers map[Reference]plugin.Provider
	m sync.RWMutex
}

// RegisterProvider loads and registers a provider for the given URN.
func (r *registry) RegisterProvider(urn resource.URN, properties resource.PropertyMap,
		allowUnknowns bool) (Reference, plugin.Provider, []plugin.CheckFailure, error) {
	r.m.Lock()
	defer r.m.Unlock()

	if err := validateURN(urn); err != nil {
		return Reference{}, nil, nilm err
	}

	id, provider, failures, err := loadProvider(r.host, urn, properties, allowUnknowns)
	switch {
	case err != nil:
		return Reference{}, nil, nil, err
	case len(failures) != 0:
		return Reference{}, nil, failures, nil
	}

	ref, err := NewReference(urn, id)
	contract.Assert(err == nil)
	r.providers[ref] = provider
	return ref, provider, nil, nil
}

// GetProvider returns the provider plugin with the given URN and ID.
func (r *registry) GetProvider(ref Reference) (plugin.Provider, bool) {
	r.m.RLock()
	defer r.m.RUnlock()

	p, ok := r.providers[ref]
	return p, ok
}

// providerInputs collects the inputs necessary to load a provider.
type providerInputs struct {
	version *semver.Version
	config  map[config.Key]string
}

// parseProperties parses a provider's version and configuraiton out of a property bag. The second return value will be
// true if any properties in the bag were unknown.
func parseProperties(pkg tokens.Package,
	properties resource.PropertyMap, allowUnknowns bool) (providerInputs, bool, []plugin.CheckFailure) {

	var failures []plugin.CheckFailure
	var version *semver.Version
	if versionProp, ok := properties["version"]; ok {
		if !versionProp.IsString() {
			failures = append(failures, plugin.CheckFailure{
				Property: "version",
				Reason:   "'version' must be a string",
			})
		} else {
			sv, err := semver.ParseTolerant(versionProp.StringValue())
			if err != nil {
				failures = append(failures, plugin.CheckFailure{
					Property: "version",
					Reason:   fmt.Sprintf("could not parse provider version: %v", err),
				})
			}
			version = &sv
		}
	}

	// Convert the property map to a provider config map, removing reserved properties.
	containsUnknowns := false
	cfg := make(map[config.Key]string)
	for k, v := range properties {
		if k == "version" {
			continue
		}

		switch {
		case v.IsComputed():
			containsUnknowns = true
			if !allowUnknowns {
				failures = append(failures, plugin.CheckFailure{
					Property: k,
					Reason:   "provider properties must not be unknown",
				})
			}
		case v.IsString():
			key := config.MustMakeKey(string(pkg), string(k))
			cfg[key] = v.StringValue()
		default:
			failures = append(failures, plugin.CheckFailure{
				Property: k,
				Reason:   "provider property values must be strings",
			})
		}
	}

	inputs := providerInputs{
		version: version,
		config: cfg,
	}
	return inputs, containsUnknowns, failures
}

// createShim creates a shim for the given provider.
func createShim(host plugin.Host, pkg tokens.Package, version *semver.Version) (plugin.Provider, error) {
	// Load the provider, get its info, and construct an appropriate shim.
	provider, err := host.Provider(pkg, version)
	if err != nil {
		return nil, err
	}
	defer func() { contract.IgnoreError(host.CloseProvider(provider)) }()

	info, err := provider.GetPluginInfo()
	if err != nil {
		return nil, err
	}

	shim := &shimProvider{
		pkg: pkg,
		info: info,
	}
	return shim, nil
}

// loadProvider loads a provider for the given URN with the indicated properties.
func loadProvider(host plugin.Host, urn resource.URN, properties resource.PropertyMap,
	allowUnknowns bool) (resource.ID, plugin.Provider, []plugin.CheckFailure, error) {

	logging.V(7).Infof("loading provider %v", urn)

	pkg := tokens.Package(urn.Type().Name())

	// Parse the property bag. If there are any validation failures, simply return them.
	inputs, hasUnknowns, failures := parseProperties(pkg, properties, allowUnknowns)
	if len(failures) != 0 {
		return "", nil, failures, nil
	}

	// If there were any unknown properties, we'll need to shim the provider. Note that if we get this far we must
	// be performing a preview and don't need to return a real ID.
	if hasUnknowns {
		contract.Assert(allowUnknowns)
		shim, err := createShim(host, pkg, inputs.version)
		if err != nil {
			return "", nil, nil, err
		}
		return "", shim, nil, err
	}

	// Finally, attempt to load and configure the provider.
	provider, err := host.Provider(pkg, inputs.version)
	if err != nil {
		return "", nil, nil, err
	}

	// If we have config, attempt to configure the plugin. If configuration fails, discard the loaded plugin.
	if err = provider.Configure(inputs.config); err != nil {
		closeErr := host.CloseProvider(provider)
		if closeErr != nil {
			logging.Infof("Error closing provider; ignoring: %v", closeErr)
		}
		return "", nil, nil, err
	}

	logging.V(7).Infof("loaded provider %v", urn)

	// Legacy providers always receive the same ID: "v0".
	return "v0", provider, nil, nil
}
