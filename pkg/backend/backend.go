// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

// Package backend encapsulates all extensibility points required to fully implement a new cloud provider.
package backend

import (
	"encoding/json"

	"github.com/pulumi/pulumi/pkg/engine"
	"github.com/pulumi/pulumi/pkg/operations"
	"github.com/pulumi/pulumi/pkg/pack"
	"github.com/pulumi/pulumi/pkg/resource/config"
	"github.com/pulumi/pulumi/pkg/tokens"
)

// Backend is an interface that represents actions the engine will interact with to manage stacks of cloud resources.
// It can be implemented any number of ways to provide pluggable backend implementations of the Pulumi Cloud.
type Backend interface {
	// Name returns a friendly name for this backend.
	Name() string

	// GetStack returns a stack object tied to this backend with the given name, or nil if it cannot be found.
	GetStack(name tokens.QName) (Stack, error)
	// CreateStack creates a new stack with the given name and options that are specific to the backend provider.
	CreateStack(name tokens.QName, opts interface{}) error
	// RemoveStack removes a stack with the given name.  If force is true, the stack will be removed even if it
	// still contains resources.  Otherwise, if the stack contains resources, a non-nil error is returned, and the
	// first boolean return value will be set to true.
	RemoveStack(name tokens.QName, force bool) (bool, error)
	// ListStacks returns a list of stack summaries for all known stacks in the target backend.
	ListStacks() ([]Stack, error)

	// GetStackCrypter returns an encrypter/decrypter for the given stack's secret config values.
	GetStackCrypter(stack tokens.QName) (config.Crypter, error)

	// Preview initiates a preview of the current workspace's contents.
	Preview(stackName tokens.QName, pkg *pack.Package, root string,
		debug bool, opts engine.UpdateOptions, displayOpts DisplayOptions) error
	// Update updates the target stack with the current workspace's contents (config and code).
	Update(stackName tokens.QName, pkg *pack.Package, root string,
		debug bool, m UpdateMetadata, opts engine.UpdateOptions, displayOpts DisplayOptions) error
	// Destroy destroys all of this stack's resources.
	Destroy(stackName tokens.QName, pkg *pack.Package, root string,
		debug bool, m UpdateMetadata, opts engine.UpdateOptions, displayOpts DisplayOptions) error

	// GetHistory returns all updates for the stack. The returned UpdateInfo slice will be in
	// descending order by Version.
	GetHistory(stackName tokens.QName) ([]UpdateInfo, error)
	// GetLogs fetches a list of log entries for the given stack, with optional filtering/querying.
	GetLogs(stackName tokens.QName, query operations.LogQuery) ([]operations.LogEntry, error)

	// ExportDeployment exports the deployment for the given stack as an opaque JSON message.
	ExportDeployment(stackName tokens.QName) (json.RawMessage, error)
	// ImportDeployment imports the given deployment into the indicated stack.
	ImportDeployment(stackName tokens.QName, deployment json.RawMessage) error
}