package providerschema

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	chalkprovider "github.com/chalk-ai/terraform-provider-chalk/internal/provider"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

type frameworkAttribute interface {
	GetType() attr.Type
	IsComputed() bool
	IsOptional() bool
	IsRequired() bool
	IsSensitive() bool
	IsWriteOnly() bool
}

// LiveSnapshot extracts the current provider contract directly from the
// Terraform Plugin Framework schemas.
func LiveSnapshot(ctx context.Context, version string) (*Snapshot, error) {
	if err := ValidateReleaseVersion(version); err != nil {
		return nil, err
	}
	instance, ok := chalkprovider.New(version)().(*chalkprovider.ChalkProvider)
	if !ok {
		return nil, fmt.Errorf("provider constructor returned an unexpected type")
	}

	snapshot := NewSnapshot(version)
	for _, constructor := range instance.Resources(ctx) {
		entity, name, err := liveResource(ctx, constructor)
		if err != nil {
			return nil, err
		}
		if _, exists := snapshot.Resources[name]; exists {
			return nil, fmt.Errorf("duplicate resource type %s", name)
		}
		snapshot.Resources[name] = entity
	}
	for _, constructor := range instance.DataSources(ctx) {
		entity, name, err := liveDataSource(ctx, constructor)
		if err != nil {
			return nil, err
		}
		if _, exists := snapshot.DataSources[name]; exists {
			return nil, fmt.Errorf("duplicate data source type %s", name)
		}
		snapshot.DataSources[name] = entity
	}
	if err := snapshot.Validate(); err != nil {
		return nil, fmt.Errorf("validating live provider schema: %w", err)
	}
	return snapshot, nil
}

func liveResource(ctx context.Context, constructor func() resource.Resource) (Entity, string, error) {
	instance := constructor()
	var metadata resource.MetadataResponse
	instance.Metadata(ctx, resource.MetadataRequest{ProviderTypeName: "chalk"}, &metadata)
	if metadata.TypeName == "" {
		return Entity{}, "", fmt.Errorf("resource returned an empty type name")
	}

	var response resource.SchemaResponse
	instance.Schema(ctx, resource.SchemaRequest{}, &response)
	if response.Diagnostics.HasError() {
		return Entity{}, "", fmt.Errorf("%s schema diagnostics: %v", metadata.TypeName, response.Diagnostics.Errors())
	}
	nodes, err := resourceNodes(response.Schema.Attributes, response.Schema.Blocks)
	if err != nil {
		return Entity{}, "", fmt.Errorf("%s schema: %w", metadata.TypeName, err)
	}
	permissions, err := ParsePermissions(chalkprovider.ResourcePermissionsMarkdown(metadata.TypeName))
	if err != nil {
		return Entity{}, "", fmt.Errorf("%s permissions: %w", metadata.TypeName, err)
	}
	return Entity{Schema: nodes, Permissions: permissions}, metadata.TypeName, nil
}

func liveDataSource(ctx context.Context, constructor func() datasource.DataSource) (Entity, string, error) {
	instance := constructor()
	var metadata datasource.MetadataResponse
	instance.Metadata(ctx, datasource.MetadataRequest{ProviderTypeName: "chalk"}, &metadata)
	if metadata.TypeName == "" {
		return Entity{}, "", fmt.Errorf("data source returned an empty type name")
	}

	var response datasource.SchemaResponse
	instance.Schema(ctx, datasource.SchemaRequest{}, &response)
	if response.Diagnostics.HasError() {
		return Entity{}, "", fmt.Errorf("%s schema diagnostics: %v", metadata.TypeName, response.Diagnostics.Errors())
	}
	nodes, err := dataSourceNodes(response.Schema.Attributes, response.Schema.Blocks)
	if err != nil {
		return Entity{}, "", fmt.Errorf("%s schema: %w", metadata.TypeName, err)
	}
	permissions, err := ParsePermissions(chalkprovider.DataSourcePermissionsMarkdown(metadata.TypeName))
	if err != nil {
		return Entity{}, "", fmt.Errorf("%s permissions: %w", metadata.TypeName, err)
	}
	return Entity{Schema: nodes, Permissions: permissions}, metadata.TypeName, nil
}

func resourceNodes(attributes map[string]rschema.Attribute, blocks map[string]rschema.Block) (map[string]Node, error) {
	nodes := make(map[string]Node, len(attributes)+len(blocks))
	for name, attribute := range attributes {
		node, err := resourceAttributeNode(attribute)
		if err != nil {
			return nil, fmt.Errorf("attribute %s: %w", name, err)
		}
		nodes[name] = node
	}
	for name, block := range blocks {
		if _, exists := nodes[name]; exists {
			return nil, fmt.Errorf("schema name %s is both an attribute and block", name)
		}
		node, err := resourceBlockNode(block)
		if err != nil {
			return nil, fmt.Errorf("block %s: %w", name, err)
		}
		nodes[name] = node
	}
	return nodes, nil
}

func resourceAttributeNode(attribute rschema.Attribute) (Node, error) {
	node, err := baseAttributeNode(attribute)
	if err != nil {
		return Node{}, err
	}
	switch typed := attribute.(type) {
	case rschema.SingleNestedAttribute:
		node.Type = nil
		node.NestingMode = "single"
		node.Children, err = resourceNodes(typed.Attributes, nil)
	case rschema.ListNestedAttribute:
		node.Type = nil
		node.NestingMode = "list"
		node.Children, err = resourceNodes(typed.NestedObject.Attributes, nil)
	case rschema.SetNestedAttribute:
		node.Type = nil
		node.NestingMode = "set"
		node.Children, err = resourceNodes(typed.NestedObject.Attributes, nil)
	case rschema.MapNestedAttribute:
		node.Type = nil
		node.NestingMode = "map"
		node.Children, err = resourceNodes(typed.NestedObject.Attributes, nil)
	}
	return node, err
}

func resourceBlockNode(block rschema.Block) (Node, error) {
	node := Node{Kind: "block"}
	var err error
	switch typed := block.(type) {
	case rschema.SingleNestedBlock:
		node.NestingMode = "single"
		node.Children, err = resourceNodes(typed.Attributes, typed.Blocks)
	case rschema.ListNestedBlock:
		node.NestingMode = "list"
		node.Children, err = resourceNodes(typed.NestedObject.Attributes, typed.NestedObject.Blocks)
	case rschema.SetNestedBlock:
		node.NestingMode = "set"
		node.Children, err = resourceNodes(typed.NestedObject.Attributes, typed.NestedObject.Blocks)
	default:
		return Node{}, fmt.Errorf("unsupported resource block type %T", block)
	}
	return node, err
}

func dataSourceNodes(attributes map[string]dschema.Attribute, blocks map[string]dschema.Block) (map[string]Node, error) {
	nodes := make(map[string]Node, len(attributes)+len(blocks))
	for name, attribute := range attributes {
		node, err := dataSourceAttributeNode(attribute)
		if err != nil {
			return nil, fmt.Errorf("attribute %s: %w", name, err)
		}
		nodes[name] = node
	}
	for name, block := range blocks {
		if _, exists := nodes[name]; exists {
			return nil, fmt.Errorf("schema name %s is both an attribute and block", name)
		}
		node, err := dataSourceBlockNode(block)
		if err != nil {
			return nil, fmt.Errorf("block %s: %w", name, err)
		}
		nodes[name] = node
	}
	return nodes, nil
}

func dataSourceAttributeNode(attribute dschema.Attribute) (Node, error) {
	node, err := baseAttributeNode(attribute)
	if err != nil {
		return Node{}, err
	}
	switch typed := attribute.(type) {
	case dschema.SingleNestedAttribute:
		node.Type = nil
		node.NestingMode = "single"
		node.Children, err = dataSourceNodes(typed.Attributes, nil)
	case dschema.ListNestedAttribute:
		node.Type = nil
		node.NestingMode = "list"
		node.Children, err = dataSourceNodes(typed.NestedObject.Attributes, nil)
	case dschema.SetNestedAttribute:
		node.Type = nil
		node.NestingMode = "set"
		node.Children, err = dataSourceNodes(typed.NestedObject.Attributes, nil)
	case dschema.MapNestedAttribute:
		node.Type = nil
		node.NestingMode = "map"
		node.Children, err = dataSourceNodes(typed.NestedObject.Attributes, nil)
	}
	return node, err
}

func dataSourceBlockNode(block dschema.Block) (Node, error) {
	node := Node{Kind: "block"}
	var err error
	switch typed := block.(type) {
	case dschema.SingleNestedBlock:
		node.NestingMode = "single"
		node.Children, err = dataSourceNodes(typed.Attributes, typed.Blocks)
	case dschema.ListNestedBlock:
		node.NestingMode = "list"
		node.Children, err = dataSourceNodes(typed.NestedObject.Attributes, typed.NestedObject.Blocks)
	case dschema.SetNestedBlock:
		node.NestingMode = "set"
		node.Children, err = dataSourceNodes(typed.NestedObject.Attributes, typed.NestedObject.Blocks)
	default:
		return Node{}, fmt.Errorf("unsupported data source block type %T", block)
	}
	return node, err
}

func baseAttributeNode(attribute frameworkAttribute) (Node, error) {
	terraformType := attribute.GetType().TerraformType(context.Background())
	rawType, err := marshalTerraformType(terraformType)
	if err != nil {
		return Node{}, fmt.Errorf("marshalling Terraform type: %w", err)
	}
	typ, err := canonicalType(rawType)
	if err != nil {
		return Node{}, err
	}
	return Node{
		Kind:      "attribute",
		Type:      typ,
		Optional:  attribute.IsOptional(),
		Required:  attribute.IsRequired(),
		Computed:  attribute.IsComputed(),
		Sensitive: attribute.IsSensitive(),
		WriteOnly: attribute.IsWriteOnly(),
	}, nil
}

func marshalTerraformType(terraformType tftypes.Type) ([]byte, error) {
	value, err := terraformTypeValue(terraformType)
	if err != nil {
		return nil, err
	}
	return json.Marshal(value)
}

func terraformTypeValue(terraformType tftypes.Type) (any, error) {
	switch {
	case terraformType.Is(tftypes.String):
		return "string", nil
	case terraformType.Is(tftypes.Number):
		return "number", nil
	case terraformType.Is(tftypes.Bool):
		return "bool", nil
	case terraformType.Is(tftypes.DynamicPseudoType):
		return "dynamic", nil
	}

	switch typed := terraformType.(type) {
	case tftypes.List:
		element, err := terraformTypeValue(typed.ElementType)
		return []any{"list", element}, err
	case tftypes.Set:
		element, err := terraformTypeValue(typed.ElementType)
		return []any{"set", element}, err
	case tftypes.Map:
		element, err := terraformTypeValue(typed.ElementType)
		return []any{"map", element}, err
	case tftypes.Tuple:
		elements := make([]any, 0, len(typed.ElementTypes))
		for _, elementType := range typed.ElementTypes {
			element, err := terraformTypeValue(elementType)
			if err != nil {
				return nil, err
			}
			elements = append(elements, element)
		}
		return []any{"tuple", elements}, nil
	case tftypes.Object:
		attributes := make(map[string]any, len(typed.AttributeTypes))
		for name, attributeType := range typed.AttributeTypes {
			attribute, err := terraformTypeValue(attributeType)
			if err != nil {
				return nil, err
			}
			attributes[name] = attribute
		}
		value := []any{"object", attributes}
		if len(typed.OptionalAttributes) > 0 {
			optional := make([]string, 0, len(typed.OptionalAttributes))
			for name := range typed.OptionalAttributes {
				optional = append(optional, name)
			}
			sort.Strings(optional)
			value = append(value, optional)
		}
		return value, nil
	default:
		return nil, fmt.Errorf("unsupported Terraform type %T", terraformType)
	}
}
