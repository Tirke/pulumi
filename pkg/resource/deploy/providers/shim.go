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
	"github.com/pkg/errors"

	"github.com/pulumi/pulumi/pkg/resource"
	"github.com/pulumi/pulumi/pkg/resource/config"
	"github.com/pulumi/pulumi/pkg/resource/plugin"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/workspace"
)

var _ plugin.Provider = (*shimProvider)(nil)

// shimProvider is a no-op implementation of plugin.Provider used to provide preview-only functionality for legacy
// providers if any of their configuration values are unknown.
type shimProvider struct {
	pkg  tokens.Package
	info workspace.PluginInfo
}

func (p *shimProvider) Close() error {
	return nil
}

func (p *shimProvider) Pkg() tokens.Package {
	return p.pkg
}

func (p *shimProvider) Configure(props map[config.Key]string) error {
	contract.Fail()
	return errors.New("the shimProvider is not configurable")
}

func (p *shimProvider) Check(urn resource.URN, olds, news resource.PropertyMap,
	allowUnknowns bool) (resource.PropertyMap, []plugin.CheckFailure, error) {

	return news, nil, nil
}

func (p *shimProvider) Diff(urn resource.URN, id resource.ID, olds, news resource.PropertyMap,
	allowUnknowns bool) (plugin.DiffResult, error) {

	// never require replacement
	return plugin.DiffResult{Changes: plugin.DiffUnknown}, nil
}

func (p *shimProvider) Create(urn resource.URN,
	news resource.PropertyMap) (resource.ID, resource.PropertyMap, resource.Status, error) {
	contract.Fail()
	return "", nil, resource.StatusOK, errors.New("the shimProvider cannot perform CRUD operations")
}

func (p *shimProvider) Read(urn resource.URN, id resource.ID,
	props resource.PropertyMap) (resource.PropertyMap, error) {
	return resource.PropertyMap{}, nil
}

func (p *shimProvider) Update(urn resource.URN, id resource.ID, olds,
	news resource.PropertyMap) (resource.PropertyMap, resource.Status, error) {
	contract.Fail()
	return nil, resource.StatusOK, errors.New("the shimProvider cannot perform CRUD operations")
}

func (p *shimProvider) Delete(urn resource.URN, id resource.ID, props resource.PropertyMap) (resource.Status, error) {
	contract.Fail()
	return resource.StatusOK, errors.New("the shimProvider cannot perform CRUD operations")
}

func (p *shimProvider) Invoke(tok tokens.ModuleMember,
	args resource.PropertyMap) (resource.PropertyMap, []plugin.CheckFailure, error) {
	return resource.PropertyMap{}, nil, nil
}

func (p *shimProvider) GetPluginInfo() (workspace.PluginInfo, error) {
	return p.info, nil
}

func (p *shimProvider) SignalCancellation() error {
	return nil
}
