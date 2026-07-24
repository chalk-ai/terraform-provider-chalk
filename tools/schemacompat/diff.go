package main

import (
	"fmt"

	"github.com/chalk-ai/terraform-provider-chalk/tools/internal/providerschema"
)

type ProviderSchemaOutput = providerschema.TerraformProviderSchemaOutput
type ProviderSchema = providerschema.TerraformProviderSchema
type SchemaEntry = providerschema.TerraformSchemaEntry
type Block = providerschema.TerraformBlock
type Attribute = providerschema.TerraformAttribute
type NestedType = providerschema.TerraformNestedType
type BlockType = providerschema.TerraformNestedBlock

// BreakingChange describes a single detected breaking change.
type BreakingChange struct {
	Rule    string
	Path    string
	Message string
}

func (b BreakingChange) String() string {
	return fmt.Sprintf("[%s] %s: %s", b.Rule, b.Path, b.Message)
}

// Diff compares two provider schema outputs and returns all detected breaking
// changes. The normalized recursive model is shared with genchangelog so
// Framework nested attributes and legacy nested blocks are treated equally.
func Diff(old, current *ProviderSchemaOutput) []BreakingChange {
	oldSnapshot, err := providerschema.SnapshotFromTerraformOutput(old, "v0.0.0")
	if err != nil {
		return []BreakingChange{{Rule: "INTERNAL", Path: "old schema", Message: err.Error()}}
	}
	currentSnapshot, err := providerschema.SnapshotFromTerraformOutput(current, "v0.0.0")
	if err != nil {
		return []BreakingChange{{Rule: "INTERNAL", Path: "new schema", Message: err.Error()}}
	}

	var breaking []BreakingChange
	for _, change := range providerschema.Diff(oldSnapshot, currentSnapshot) {
		oldNode, oldNodeFound := nodeAt(oldSnapshot, change.EntityKind, change.Entity, change.Path)
		currentNode, currentNodeFound := nodeAt(currentSnapshot, change.EntityKind, change.Entity, change.Path)
		path := displayPath(change)

		switch change.Kind {
		case providerschema.ChangeEntityRemoved:
			rule := "R001"
			message := "resource was deleted"
			if change.EntityKind == "data_source" {
				rule = "R002"
				message = "data source was deleted"
			}
			breaking = append(breaking, BreakingChange{Rule: rule, Path: path, Message: message})
		case providerschema.ChangeNodeRemoved:
			rule := "R003"
			message := "attribute was deleted"
			if oldNodeFound && oldNode.Kind == "block" {
				rule = "R004"
				message = "block was deleted"
			}
			breaking = append(breaking, BreakingChange{Rule: rule, Path: path, Message: message})
		case providerschema.ChangeType, providerschema.ChangeNesting, providerschema.ChangeNodeKind:
			breaking = append(breaking, BreakingChange{
				Rule:    "R005",
				Path:    path,
				Message: fmt.Sprintf("type changed from %s to %s", change.Before, change.After),
			})
		case providerschema.ChangeRequired:
			if change.Before == "false" && change.After == "true" && currentNodeFound && currentNode.Kind == "attribute" {
				breaking = append(breaking, BreakingChange{
					Rule:    "R006",
					Path:    path,
					Message: "attribute changed to required",
				})
			}
		case providerschema.ChangeMinItems:
			if oldNodeFound && currentNodeFound && oldNode.MinItems == 0 && currentNode.MinItems > 0 {
				breaking = append(breaking, BreakingChange{
					Rule:    "R007",
					Path:    path,
					Message: fmt.Sprintf("block changed from optional to required (min_items=%d)", currentNode.MinItems),
				})
			}
		case providerschema.ChangeNodeAdded:
			if !currentNodeFound {
				continue
			}
			if currentNode.Kind == "attribute" && currentNode.Required {
				breaking = append(breaking, BreakingChange{
					Rule:    "R008",
					Path:    path,
					Message: "new required attribute was added",
				})
			}
			if currentNode.Kind == "block" && currentNode.MinItems > 0 {
				breaking = append(breaking, BreakingChange{
					Rule:    "R009",
					Path:    path,
					Message: fmt.Sprintf("new required block was added (min_items=%d)", currentNode.MinItems),
				})
			}
		}
	}
	return breaking
}

func nodeAt(snapshot *providerschema.Snapshot, entityKind, entity, path string) (providerschema.Node, bool) {
	var entities map[string]providerschema.Entity
	if entityKind == "data_source" {
		entities = snapshot.DataSources
	} else {
		entities = snapshot.Resources
	}
	currentEntity, exists := entities[entity]
	if !exists {
		return providerschema.Node{}, false
	}
	nodes := currentEntity.Schema
	var node providerschema.Node
	for _, part := range splitPath(path) {
		node, exists = nodes[part]
		if !exists {
			return providerschema.Node{}, false
		}
		nodes = node.Children
	}
	return node, path != ""
}

func splitPath(path string) []string {
	if path == "" {
		return nil
	}
	var parts []string
	start := 0
	for index := range path {
		if path[index] == '.' {
			parts = append(parts, path[start:index])
			start = index + 1
		}
	}
	return append(parts, path[start:])
}

func displayPath(change providerschema.Change) string {
	prefix := "resource "
	if change.EntityKind == "data_source" {
		prefix = "data source "
	}
	path := prefix + change.Entity
	if change.Path != "" {
		path += "." + change.Path
	}
	return path
}
