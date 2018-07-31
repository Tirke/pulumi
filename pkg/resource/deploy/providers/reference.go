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

	"github.com/pulumi/pulumi/pkg/resource"
)

// A provider reference is (URN, ID) tuple that refers to a particular provider instance. A provider reference's
// stirng representation is <URN> "::" <ID>.
type Reference struct {
	URN resource.URN
	ID  resource.ID
}

// String returns the string representation of this provider reference.
func (p Reference) String() string {
	return string(p.URN) + resource.URNNameDelimiter + string(p.ID)
}

// ParseReference parses the URN and ID from the string representation of a provider reference. If parsing was
// not possible, this function returns false.
func ParseReference(s string) (Reference, bool) {
	// If this is not a valid URN + ID, return false. Note that we don't try terribly hard to validate the URN portion
	// of the reference.
	lastSep := strings.LastIndex(s, resource.URNNameDelimiter)
	if lastSep == -1 {
		return Reference{}, false
	}
	return Reference{
		URN: resource.URN(s[:lastSep]),
		ID:  resource.ID(s[lastSep+len(resource.URNNameDelimiter):]),
	}, true
}
