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
	"strings"

	"github.com/pkg/errors"

	"github.com/pulumi/pulumi/pkg/resource"
	"github.com/pulumi/pulumi/pkg/tokens"
)

// IsProviderType returns true if the supplied type token refers to a Pulumi provider.
func IsProviderType(typ tokens.Type) bool {
	return typ.Package() == "pulumi" && typ.Module() == "providers" && typ.Name() != ""
}

func validateURN(urn resource.URN) error {
	typ := urn.Type()
	if typ.Package() != "pulumi" {
		return errors.Errorf("invalid package in type: expected 'pulumi', got '%v'", typ.Package())
	}
	if typ.Module() != "providers" {
		return errors.Errorf("invaid module in type: expected 'providers', got '%v'", typ.Module())
	}
	if typ.Name() == "" {
		return errors.New("provider URNs must specify a type name")
	}
	return nil
}

// A provider reference is (URN, ID) tuple that refers to a particular provider instance. A provider reference's
// stirng representation is <URN> "::" <ID>. The URN's type portion must be of the form "pulumi:providers:<pkg>".
type Reference struct {
	urn resource.URN
	id  resource.ID
}

// String returns the string representation of this provider reference.
func (p Reference) String() string {
	return string(p.URN) + resource.URNNameDelimiter + string(p.ID)
}

// NewReference creates a new reference for the given URN and ID.
func NewReference(urn resource.URN, id resource.ID) (Reference, error) {
	if err := validateURN(urn); err != nil {
		return Reference{}, err
	}
	return Reference{urn: urn, id: id}, nil
}

// ParseReference parses the URN and ID from the string representation of a provider reference. If parsing was
// not possible, this function returns false.
func ParseReference(s string) (Reference, error) {
	// If this is not a valid URN + ID, return false. Note that we don't try terribly hard to validate the URN portion
	// of the reference.
	lastSep := strings.LastIndex(s, resource.URNNameDelimiter)
	if lastSep == -1 {
		return Reference{}, errors.New("expected '%v' in provider reference '%v'", resource.URNNameDelimiter, s)
	}
	urn, id := resource.URN(s[:lastSep]), resource.ID(s[lastSep+len(resource.URNNameDelimiter):])
	if err := validateURN(urn); err != nil {
		return Reference{}, err
	}
	return Reference{urn: urn, id: id}, nil
}
