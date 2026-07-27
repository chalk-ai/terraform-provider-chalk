package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	chalkprovider "github.com/chalk-ai/terraform-provider-chalk/internal/provider"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

var versionPattern = regexp.MustCompile(`^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)

type Snapshot struct {
	Version     string            `json:"version"`
	Resources   map[string]Entity `json:"resources"`
	DataSources map[string]Entity `json:"data_sources"`
}

type Entity struct {
	Attributes  map[string]Attribute `json:"attributes"`
	Permissions string               `json:"permissions,omitempty"`
}

type Attribute struct {
	Type     string `json:"type"`
	Required bool   `json:"required,omitempty"`
}

type frameworkAttribute interface {
	GetType() attr.Type
	IsRequired() bool
}

func liveSnapshot(ctx context.Context, version string) (Snapshot, error) {
	if _, err := versionParts(version); err != nil {
		return Snapshot{}, err
	}

	instance := chalkprovider.New(version)()
	snapshot := Snapshot{
		Version:     version,
		Resources:   map[string]Entity{},
		DataSources: map[string]Entity{},
	}

	for _, constructor := range instance.Resources(ctx) {
		name, entity, err := snapshotResource(ctx, constructor)
		if err != nil {
			return Snapshot{}, err
		}
		snapshot.Resources[name] = entity
	}
	for _, constructor := range instance.DataSources(ctx) {
		name, entity, err := snapshotDataSource(ctx, constructor)
		if err != nil {
			return Snapshot{}, err
		}
		snapshot.DataSources[name] = entity
	}
	return snapshot, nil
}

func snapshotResource(ctx context.Context, constructor func() resource.Resource) (string, Entity, error) {
	instance := constructor()
	var metadata resource.MetadataResponse
	instance.Metadata(ctx, resource.MetadataRequest{ProviderTypeName: "chalk"}, &metadata)

	var response resource.SchemaResponse
	instance.Schema(ctx, resource.SchemaRequest{}, &response)
	if response.Diagnostics.HasError() {
		return "", Entity{}, fmt.Errorf("%s schema: %v", metadata.TypeName, response.Diagnostics.Errors())
	}

	attributes := map[string]Attribute{}
	if err := collectResourceSchema(ctx, attributes, "", response.Schema.Attributes, response.Schema.Blocks); err != nil {
		return "", Entity{}, fmt.Errorf("%s schema: %w", metadata.TypeName, err)
	}
	return metadata.TypeName, Entity{
		Attributes:  attributes,
		Permissions: chalkprovider.ResourcePermissionsMarkdown(metadata.TypeName),
	}, nil
}

func snapshotDataSource(ctx context.Context, constructor func() datasource.DataSource) (string, Entity, error) {
	instance := constructor()
	var metadata datasource.MetadataResponse
	instance.Metadata(ctx, datasource.MetadataRequest{ProviderTypeName: "chalk"}, &metadata)

	var response datasource.SchemaResponse
	instance.Schema(ctx, datasource.SchemaRequest{}, &response)
	if response.Diagnostics.HasError() {
		return "", Entity{}, fmt.Errorf("%s schema: %v", metadata.TypeName, response.Diagnostics.Errors())
	}

	attributes := map[string]Attribute{}
	if err := collectDataSourceSchema(ctx, attributes, "", response.Schema.Attributes, response.Schema.Blocks); err != nil {
		return "", Entity{}, fmt.Errorf("%s schema: %w", metadata.TypeName, err)
	}
	return metadata.TypeName, Entity{
		Attributes:  attributes,
		Permissions: chalkprovider.DataSourcePermissionsMarkdown(metadata.TypeName),
	}, nil
}

func collectResourceSchema(ctx context.Context, result map[string]Attribute, prefix string, attributes map[string]rschema.Attribute, blocks map[string]rschema.Block) error {
	for name, attribute := range attributes {
		path := joinPath(prefix, name)
		attributeType := formatTerraformType(attribute.GetType().TerraformType(ctx))
		var nested map[string]rschema.Attribute
		switch value := attribute.(type) {
		case rschema.SingleNestedAttribute:
			attributeType, nested = "object", value.Attributes
		case rschema.ListNestedAttribute:
			attributeType, nested = "list(object)", value.NestedObject.Attributes
		case rschema.SetNestedAttribute:
			attributeType, nested = "set(object)", value.NestedObject.Attributes
		case rschema.MapNestedAttribute:
			attributeType, nested = "map(object)", value.NestedObject.Attributes
		}
		result[path] = trackedAttribute(attribute, attributeType)
		if err := collectResourceSchema(ctx, result, path, nested, nil); err != nil {
			return err
		}
	}

	for name, block := range blocks {
		path := joinPath(prefix, name)
		switch value := block.(type) {
		case rschema.SingleNestedBlock:
			if err := collectResourceSchema(ctx, result, path, value.Attributes, value.Blocks); err != nil {
				return err
			}
		case rschema.ListNestedBlock:
			if err := collectResourceSchema(ctx, result, path, value.NestedObject.Attributes, value.NestedObject.Blocks); err != nil {
				return err
			}
		case rschema.SetNestedBlock:
			if err := collectResourceSchema(ctx, result, path, value.NestedObject.Attributes, value.NestedObject.Blocks); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unsupported block type %T", block)
		}
	}
	return nil
}

func collectDataSourceSchema(ctx context.Context, result map[string]Attribute, prefix string, attributes map[string]dschema.Attribute, blocks map[string]dschema.Block) error {
	for name, attribute := range attributes {
		path := joinPath(prefix, name)
		attributeType := formatTerraformType(attribute.GetType().TerraformType(ctx))
		var nested map[string]dschema.Attribute
		switch value := attribute.(type) {
		case dschema.SingleNestedAttribute:
			attributeType, nested = "object", value.Attributes
		case dschema.ListNestedAttribute:
			attributeType, nested = "list(object)", value.NestedObject.Attributes
		case dschema.SetNestedAttribute:
			attributeType, nested = "set(object)", value.NestedObject.Attributes
		case dschema.MapNestedAttribute:
			attributeType, nested = "map(object)", value.NestedObject.Attributes
		}
		result[path] = trackedAttribute(attribute, attributeType)
		if err := collectDataSourceSchema(ctx, result, path, nested, nil); err != nil {
			return err
		}
	}

	for name, block := range blocks {
		path := joinPath(prefix, name)
		switch value := block.(type) {
		case dschema.SingleNestedBlock:
			if err := collectDataSourceSchema(ctx, result, path, value.Attributes, value.Blocks); err != nil {
				return err
			}
		case dschema.ListNestedBlock:
			if err := collectDataSourceSchema(ctx, result, path, value.NestedObject.Attributes, value.NestedObject.Blocks); err != nil {
				return err
			}
		case dschema.SetNestedBlock:
			if err := collectDataSourceSchema(ctx, result, path, value.NestedObject.Attributes, value.NestedObject.Blocks); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unsupported block type %T", block)
		}
	}
	return nil
}

func trackedAttribute(attribute frameworkAttribute, attributeType string) Attribute {
	return Attribute{Type: attributeType, Required: attribute.IsRequired()}
}

func formatTerraformType(terraformType tftypes.Type) string {
	switch {
	case terraformType.Is(tftypes.String):
		return "string"
	case terraformType.Is(tftypes.Number):
		return "number"
	case terraformType.Is(tftypes.Bool):
		return "bool"
	case terraformType.Is(tftypes.DynamicPseudoType):
		return "dynamic"
	}

	switch value := terraformType.(type) {
	case tftypes.List:
		return "list(" + formatTerraformType(value.ElementType) + ")"
	case tftypes.Set:
		return "set(" + formatTerraformType(value.ElementType) + ")"
	case tftypes.Map:
		return "map(" + formatTerraformType(value.ElementType) + ")"
	case tftypes.Tuple:
		elements := make([]string, len(value.ElementTypes))
		for index, elementType := range value.ElementTypes {
			elements[index] = formatTerraformType(elementType)
		}
		return "tuple([" + strings.Join(elements, ", ") + "])"
	case tftypes.Object:
		names := make([]string, 0, len(value.AttributeTypes))
		for name := range value.AttributeTypes {
			names = append(names, name)
		}
		sort.Strings(names)

		attributes := make([]string, 0, len(names))
		for _, name := range names {
			attributeType := formatTerraformType(value.AttributeTypes[name])
			if _, optional := value.OptionalAttributes[name]; optional {
				attributeType = "optional(" + attributeType + ")"
			}
			attributes = append(attributes, name+" = "+attributeType)
		}
		return "object({" + strings.Join(attributes, ", ") + "})"
	default:
		return terraformType.String()
	}
}

func joinPath(prefix, name string) string {
	if prefix == "" {
		return name
	}
	return prefix + "." + name
}

func writeSnapshot(directory string, snapshot Snapshot) error {
	if _, err := versionParts(snapshot.Version); err != nil {
		return err
	}
	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(directory, snapshot.Version+".json"), append(data, '\n'), 0o644)
}

func loadSnapshots(directory string) ([]Snapshot, error) {
	paths, err := filepath.Glob(filepath.Join(directory, "v*.json"))
	if err != nil {
		return nil, err
	}
	snapshots := make([]Snapshot, 0, len(paths))
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		var snapshot Snapshot
		if err := json.Unmarshal(data, &snapshot); err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		if _, err := versionParts(snapshot.Version); err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		if strings.TrimSuffix(filepath.Base(path), ".json") != snapshot.Version {
			return nil, fmt.Errorf("%s: filename and version do not match", path)
		}
		snapshots = append(snapshots, snapshot)
	}
	sort.Slice(snapshots, func(i, j int) bool {
		left, _ := versionParts(snapshots[i].Version)
		right, _ := versionParts(snapshots[j].Version)
		for index := range left {
			if left[index] != right[index] {
				return left[index] < right[index]
			}
		}
		return false
	})
	return snapshots, nil
}

func versionParts(version string) ([3]int, error) {
	match := versionPattern.FindStringSubmatch(version)
	if match == nil {
		return [3]int{}, fmt.Errorf("version %q must match vMAJOR.MINOR.PATCH", version)
	}
	var result [3]int
	for index := range result {
		value, err := strconv.Atoi(match[index+1])
		if err != nil {
			return [3]int{}, err
		}
		result[index] = value
	}
	return result, nil
}
