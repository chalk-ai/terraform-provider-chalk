package main

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// ProviderSchemaOutput is the top-level structure of `terraform providers schema -json`.
type ProviderSchemaOutput struct {
	FormatVersion   string                     `json:"format_version"`
	ProviderSchemas map[string]*ProviderSchema `json:"provider_schemas"`
}

// ProviderSchema holds the resource and data source schemas for a single provider.
type ProviderSchema struct {
	ResourceSchemas   map[string]*SchemaEntry `json:"resource_schemas"`
	DataSourceSchemas map[string]*SchemaEntry `json:"data_source_schemas"`
}

// SchemaEntry is a versioned block schema for a resource or data source.
type SchemaEntry struct {
	Version int   `json:"version"`
	Block   Block `json:"block"`
}

// Block contains attributes and nested block types.
type Block struct {
	Attributes map[string]*Attribute `json:"attributes"`
	BlockTypes map[string]*BlockType `json:"block_types"`
}

// Attribute represents a single attribute in a block.
type Attribute struct {
	Type     json.RawMessage `json:"type"`
	Optional bool            `json:"optional"`
	Required bool            `json:"required"`
	Computed bool            `json:"computed"`
}

// BlockType represents a nested block definition.
type BlockType struct {
	NestingMode string `json:"nesting_mode"`
	Block       Block  `json:"block"`
	MinItems    int    `json:"min_items"`
	MaxItems    int    `json:"max_items"`
}

// BreakingChange describes a single detected breaking change.
type BreakingChange struct {
	Rule    string
	Path    string
	Message string
}

func (b BreakingChange) String() string {
	return fmt.Sprintf("[%s] %s: %s", b.Rule, b.Path, b.Message)
}

// Diff compares two provider schema outputs and returns all detected breaking changes.
func Diff(old, cur *ProviderSchemaOutput) []BreakingChange {
	var changes []BreakingChange
	for addr, oldProvider := range old.ProviderSchemas {
		curProvider, ok := cur.ProviderSchemas[addr]
		if !ok {
			continue
		}
		changes = append(changes, diffProvider(oldProvider, curProvider)...)
	}
	return changes
}

func diffProvider(old, cur *ProviderSchema) []BreakingChange {
	var changes []BreakingChange

	// R001: resource deleted; diff surviving resources
	for name, oldEntry := range old.ResourceSchemas {
		curEntry, ok := cur.ResourceSchemas[name]
		if !ok {
			changes = append(changes, BreakingChange{
				Rule:    "R001",
				Path:    "resource " + name,
				Message: "resource was deleted",
			})
			continue
		}
		changes = append(changes, diffBlock("resource "+name, &oldEntry.Block, &curEntry.Block)...)
	}

	// R002: data source deleted; diff surviving data sources
	for name, oldEntry := range old.DataSourceSchemas {
		curEntry, ok := cur.DataSourceSchemas[name]
		if !ok {
			changes = append(changes, BreakingChange{
				Rule:    "R002",
				Path:    "data source " + name,
				Message: "data source was deleted",
			})
			continue
		}
		changes = append(changes, diffBlock("data source "+name, &oldEntry.Block, &curEntry.Block)...)
	}

	return changes
}

func diffBlock(path string, old, cur *Block) []BreakingChange {
	var changes []BreakingChange

	// R003: attribute deleted; R005/R006: attribute modified
	for name, oldAttr := range old.Attributes {
		curAttr, ok := cur.Attributes[name]
		if !ok {
			changes = append(changes, BreakingChange{
				Rule:    "R003",
				Path:    path + "." + name,
				Message: "attribute was deleted",
			})
			continue
		}
		changes = append(changes, diffAttribute(path+"."+name, oldAttr, curAttr)...)
	}

	// R008: new required attribute added
	for name, curAttr := range cur.Attributes {
		if _, ok := old.Attributes[name]; !ok && curAttr.Required {
			changes = append(changes, BreakingChange{
				Rule:    "R008",
				Path:    path + "." + name,
				Message: "new required attribute was added",
			})
		}
	}

	// R004: block deleted; recurse into surviving blocks
	for name, oldBlock := range old.BlockTypes {
		curBlock, ok := cur.BlockTypes[name]
		if !ok {
			changes = append(changes, BreakingChange{
				Rule:    "R004",
				Path:    path + "." + name,
				Message: "block was deleted",
			})
			continue
		}
		changes = append(changes, diffBlockType(path+"."+name, oldBlock, curBlock)...)
	}

	// R009: new required block added
	for name, curBlock := range cur.BlockTypes {
		if _, ok := old.BlockTypes[name]; !ok && curBlock.MinItems > 0 {
			changes = append(changes, BreakingChange{
				Rule:    "R009",
				Path:    path + "." + name,
				Message: fmt.Sprintf("new required block was added (min_items=%d)", curBlock.MinItems),
			})
		}
	}

	return changes
}

func diffAttribute(path string, old, cur *Attribute) []BreakingChange {
	var changes []BreakingChange

	// R005: attribute type changed
	if !jsonEqual(old.Type, cur.Type) {
		changes = append(changes, BreakingChange{
			Rule:    "R005",
			Path:    path,
			Message: fmt.Sprintf("attribute type changed from %s to %s", old.Type, cur.Type),
		})
	}

	// R006: optional → required, or computed-only → required
	if !old.Required && cur.Required && (old.Optional || old.Computed) {
		msg := "attribute changed from optional to required"
		if old.Computed && !old.Optional {
			msg = "attribute changed from computed-only to required"
		}
		changes = append(changes, BreakingChange{
			Rule:    "R006",
			Path:    path,
			Message: msg,
		})
	}

	return changes
}

func diffBlockType(path string, old, cur *BlockType) []BreakingChange {
	var changes []BreakingChange

	// R007: optional block → required
	if old.MinItems == 0 && cur.MinItems > 0 {
		changes = append(changes, BreakingChange{
			Rule:    "R007",
			Path:    path,
			Message: fmt.Sprintf("block changed from optional to required (min_items=%d)", cur.MinItems),
		})
	}

	changes = append(changes, diffBlock(path, &old.Block, &cur.Block)...)
	return changes
}

// jsonEqual compares two json.RawMessage values for semantic equality.
func jsonEqual(a, b json.RawMessage) bool {
	if a == nil && b == nil {
		return true
	}
	return bytes.Equal(normalizeJSON(a), normalizeJSON(b))
}

// normalizeJSON re-encodes a JSON value to normalize whitespace and key order.
func normalizeJSON(raw json.RawMessage) []byte {
	if raw == nil {
		return nil
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return raw
	}
	out, err := json.Marshal(v)
	if err != nil {
		return raw
	}
	return out
}
