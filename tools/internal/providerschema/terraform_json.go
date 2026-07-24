package providerschema

import (
	"encoding/json"
	"errors"
	"fmt"
)

// TerraformProviderSchemaOutput is the output envelope from
// `terraform providers schema -json`.
type TerraformProviderSchemaOutput struct {
	FormatVersion   string                              `json:"format_version"`
	ProviderSchemas map[string]*TerraformProviderSchema `json:"provider_schemas"`
}

type TerraformProviderSchema struct {
	ResourceSchemas   map[string]*TerraformSchemaEntry `json:"resource_schemas"`
	DataSourceSchemas map[string]*TerraformSchemaEntry `json:"data_source_schemas"`
}

type TerraformSchemaEntry struct {
	Version int            `json:"version"`
	Block   TerraformBlock `json:"block"`
}

type TerraformBlock struct {
	Description string                           `json:"description"`
	Attributes  map[string]*TerraformAttribute   `json:"attributes"`
	BlockTypes  map[string]*TerraformNestedBlock `json:"block_types"`
}

type TerraformAttribute struct {
	Type       json.RawMessage      `json:"type"`
	NestedType *TerraformNestedType `json:"nested_type"`
	Optional   bool                 `json:"optional"`
	Required   bool                 `json:"required"`
	Computed   bool                 `json:"computed"`
	Sensitive  bool                 `json:"sensitive"`
	WriteOnly  bool                 `json:"write_only"`
}

type TerraformNestedType struct {
	Attributes  map[string]*TerraformAttribute `json:"attributes"`
	NestingMode string                         `json:"nesting_mode"`
}

type TerraformNestedBlock struct {
	NestingMode string         `json:"nesting_mode"`
	Block       TerraformBlock `json:"block"`
	MinItems    int            `json:"min_items"`
	MaxItems    int            `json:"max_items"`
}

// SnapshotFromTerraformJSON normalizes Terraform CLI schema output. It expects
// exactly one provider because snapshots represent one provider release.
func SnapshotFromTerraformJSON(data []byte, version string) (*Snapshot, error) {
	if err := ValidateReleaseVersion(version); err != nil {
		return nil, err
	}
	var output TerraformProviderSchemaOutput
	if err := json.Unmarshal(data, &output); err != nil {
		return nil, fmt.Errorf("parsing Terraform provider schema: %w", err)
	}
	return SnapshotFromTerraformOutput(&output, version)
}

// SnapshotFromTerraformOutput normalizes parsed Terraform CLI schema output.
func SnapshotFromTerraformOutput(output *TerraformProviderSchemaOutput, version string) (*Snapshot, error) {
	if err := ValidateReleaseVersion(version); err != nil {
		return nil, err
	}
	if output == nil {
		return nil, errors.New("terraform provider schema is nil")
	}
	if len(output.ProviderSchemas) != 1 {
		return nil, fmt.Errorf("expected exactly one provider schema, found %d", len(output.ProviderSchemas))
	}

	var provider *TerraformProviderSchema
	for _, candidate := range output.ProviderSchemas {
		provider = candidate
	}
	if provider == nil {
		return nil, errors.New("provider schema is null")
	}
	snapshot := NewSnapshot(version)
	var err error
	snapshot.Resources, err = normalizeTerraformEntities(provider.ResourceSchemas)
	if err != nil {
		return nil, fmt.Errorf("normalizing resources: %w", err)
	}
	snapshot.DataSources, err = normalizeTerraformEntities(provider.DataSourceSchemas)
	if err != nil {
		return nil, fmt.Errorf("normalizing data sources: %w", err)
	}
	if err := snapshot.Validate(); err != nil {
		return nil, fmt.Errorf("validating Terraform provider schema: %w", err)
	}
	return snapshot, nil
}

func normalizeTerraformEntities(entries map[string]*TerraformSchemaEntry) (map[string]Entity, error) {
	entities := make(map[string]Entity, len(entries))
	for name, entry := range entries {
		if entry == nil {
			return nil, fmt.Errorf("%s schema is null", name)
		}
		schema, err := normalizeTerraformBlock(entry.Block)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		permissions, err := ParsePermissions(entry.Block.Description)
		if err != nil {
			return nil, fmt.Errorf("%s permissions: %w", name, err)
		}
		entities[name] = Entity{Schema: schema, Permissions: permissions}
	}
	return entities, nil
}

func normalizeTerraformBlock(block TerraformBlock) (map[string]Node, error) {
	nodes := make(map[string]Node, len(block.Attributes)+len(block.BlockTypes))
	for name, attribute := range block.Attributes {
		if attribute == nil {
			return nil, fmt.Errorf("attribute %s is null", name)
		}
		node, err := normalizeTerraformAttribute(*attribute)
		if err != nil {
			return nil, fmt.Errorf("attribute %s: %w", name, err)
		}
		nodes[name] = node
	}
	for name, nestedBlock := range block.BlockTypes {
		if nestedBlock == nil {
			return nil, fmt.Errorf("block %s is null", name)
		}
		if _, exists := nodes[name]; exists {
			return nil, fmt.Errorf("schema name %s is both an attribute and block", name)
		}
		children, err := normalizeTerraformBlock(nestedBlock.Block)
		if err != nil {
			return nil, fmt.Errorf("block %s: %w", name, err)
		}
		nodes[name] = Node{
			Kind:        "block",
			NestingMode: nestedBlock.NestingMode,
			MinItems:    nestedBlock.MinItems,
			MaxItems:    nestedBlock.MaxItems,
			Children:    children,
		}
	}
	return nodes, nil
}

func normalizeTerraformAttribute(attribute TerraformAttribute) (Node, error) {
	node := Node{
		Kind:      "attribute",
		Optional:  attribute.Optional,
		Required:  attribute.Required,
		Computed:  attribute.Computed,
		Sensitive: attribute.Sensitive,
		WriteOnly: attribute.WriteOnly,
	}
	if attribute.NestedType != nil {
		node.NestingMode = attribute.NestedType.NestingMode
		node.Children = make(map[string]Node, len(attribute.NestedType.Attributes))
		for name, child := range attribute.NestedType.Attributes {
			if child == nil {
				return Node{}, fmt.Errorf("nested attribute %s is null", name)
			}
			normalized, err := normalizeTerraformAttribute(*child)
			if err != nil {
				return Node{}, fmt.Errorf("nested attribute %s: %w", name, err)
			}
			node.Children[name] = normalized
		}
		return node, nil
	}
	typ, err := canonicalType(attribute.Type)
	if err != nil {
		return Node{}, err
	}
	if len(typ) == 0 {
		return Node{}, errors.New("attribute has neither type nor nested_type")
	}
	node.Type = typ
	return node, nil
}
