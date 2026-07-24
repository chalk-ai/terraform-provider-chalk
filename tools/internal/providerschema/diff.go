package providerschema

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

const (
	ChangeEntityAdded       = "entity_added"
	ChangeEntityRemoved     = "entity_removed"
	ChangeNodeAdded         = "node_added"
	ChangeNodeRemoved       = "node_removed"
	ChangeNodeKind          = "node_kind_changed"
	ChangeType              = "type_changed"
	ChangeNesting           = "nesting_changed"
	ChangeOptional          = "optional_changed"
	ChangeRequired          = "required_changed"
	ChangeComputed          = "computed_changed"
	ChangeSensitive         = "sensitive_changed"
	ChangeWriteOnly         = "write_only_changed"
	ChangeMinItems          = "min_items_changed"
	ChangeMaxItems          = "max_items_changed"
	ChangePermissionAdded   = "permission_added"
	ChangePermissionRemoved = "permission_removed"
	ChangePermissionScope   = "permission_scope_changed"
)

// Change is one schema or permission difference between snapshots.
type Change struct {
	EntityKind string `json:"entity_kind"`
	Entity     string `json:"entity"`
	Path       string `json:"path,omitempty"`
	Kind       string `json:"kind"`
	Before     string `json:"before,omitempty"`
	After      string `json:"after,omitempty"`
}

// Diff returns every public provider-contract change in deterministic order.
func Diff(old, current *Snapshot) []Change {
	var changes []Change
	changes = append(changes, diffEntities("resource", old.Resources, current.Resources)...)
	changes = append(changes, diffEntities("data_source", old.DataSources, current.DataSources)...)
	sortChanges(changes)
	return changes
}

func diffEntities(entityKind string, old, current map[string]Entity) []Change {
	var changes []Change
	for name, oldEntity := range old {
		currentEntity, exists := current[name]
		if !exists {
			changes = append(changes, Change{EntityKind: entityKind, Entity: name, Kind: ChangeEntityRemoved})
			continue
		}
		changes = append(changes, diffNodes(entityKind, name, "", oldEntity.Schema, currentEntity.Schema)...)
		changes = append(changes, diffPermissions(entityKind, name, oldEntity.Permissions, currentEntity.Permissions)...)
	}
	for name := range current {
		if _, exists := old[name]; !exists {
			changes = append(changes, Change{EntityKind: entityKind, Entity: name, Kind: ChangeEntityAdded})
		}
	}
	return changes
}

func diffNodes(entityKind, entity, prefix string, old, current map[string]Node) []Change {
	var changes []Change
	for name, oldNode := range old {
		path := joinPath(prefix, name)
		currentNode, exists := current[name]
		if !exists {
			changes = append(changes, Change{
				EntityKind: entityKind,
				Entity:     entity,
				Path:       path,
				Kind:       ChangeNodeRemoved,
				Before:     NodeSummary(oldNode),
			})
			continue
		}
		changes = append(changes, diffNode(entityKind, entity, path, oldNode, currentNode)...)
	}
	for name, currentNode := range current {
		if _, exists := old[name]; !exists {
			changes = append(changes, Change{
				EntityKind: entityKind,
				Entity:     entity,
				Path:       joinPath(prefix, name),
				Kind:       ChangeNodeAdded,
				After:      NodeSummary(currentNode),
			})
		}
	}
	return changes
}

func diffNode(entityKind, entity, path string, old, current Node) []Change {
	if old.Kind != current.Kind {
		return []Change{{
			EntityKind: entityKind,
			Entity:     entity,
			Path:       path,
			Kind:       ChangeNodeKind,
			Before:     old.Kind,
			After:      current.Kind,
		}}
	}
	var changes []Change
	add := func(kind, before, after string) {
		if before != after {
			changes = append(changes, Change{
				EntityKind: entityKind,
				Entity:     entity,
				Path:       path,
				Kind:       kind,
				Before:     before,
				After:      after,
			})
		}
	}
	if !bytes.Equal(old.Type, current.Type) {
		add(ChangeType, FormatType(old.Type), FormatType(current.Type))
	}
	add(ChangeNesting, old.NestingMode, current.NestingMode)
	add(ChangeOptional, boolString(old.Optional), boolString(current.Optional))
	add(ChangeRequired, boolString(old.Required), boolString(current.Required))
	add(ChangeComputed, boolString(old.Computed), boolString(current.Computed))
	add(ChangeSensitive, boolString(old.Sensitive), boolString(current.Sensitive))
	add(ChangeWriteOnly, boolString(old.WriteOnly), boolString(current.WriteOnly))
	add(ChangeMinItems, fmt.Sprint(old.MinItems), fmt.Sprint(current.MinItems))
	add(ChangeMaxItems, fmt.Sprint(old.MaxItems), fmt.Sprint(current.MaxItems))
	changes = append(changes, diffNodes(entityKind, entity, path, old.Children, current.Children)...)
	return changes
}

func diffPermissions(entityKind, entity string, old, current []Permission) []Change {
	oldByName := make(map[string]Permission, len(old))
	currentByName := make(map[string]Permission, len(current))
	for _, permission := range old {
		oldByName[permission.Name] = permission
	}
	for _, permission := range current {
		currentByName[permission.Name] = permission
	}
	var changes []Change
	for name, oldPermission := range oldByName {
		currentPermission, exists := currentByName[name]
		if !exists {
			changes = append(changes, Change{
				EntityKind: entityKind,
				Entity:     entity,
				Path:       name,
				Kind:       ChangePermissionRemoved,
				Before:     permissionScope(oldPermission),
			})
			continue
		}
		if oldPermission.TeamScoped != currentPermission.TeamScoped {
			changes = append(changes, Change{
				EntityKind: entityKind,
				Entity:     entity,
				Path:       name,
				Kind:       ChangePermissionScope,
				Before:     permissionScope(oldPermission),
				After:      permissionScope(currentPermission),
			})
		}
	}
	for name, permission := range currentByName {
		if _, exists := oldByName[name]; !exists {
			changes = append(changes, Change{
				EntityKind: entityKind,
				Entity:     entity,
				Path:       name,
				Kind:       ChangePermissionAdded,
				After:      permissionScope(permission),
			})
		}
	}
	return changes
}

func sortChanges(changes []Change) {
	sort.Slice(changes, func(i, j int) bool {
		left, right := changes[i], changes[j]
		if left.EntityKind != right.EntityKind {
			return entityKindOrder(left.EntityKind) < entityKindOrder(right.EntityKind)
		}
		if left.Entity != right.Entity {
			return left.Entity < right.Entity
		}
		if left.Path != right.Path {
			return left.Path < right.Path
		}
		return left.Kind < right.Kind
	})
}

func entityKindOrder(kind string) int {
	if kind == "resource" {
		return 0
	}
	return 1
}

func joinPath(prefix, name string) string {
	if prefix == "" {
		return name
	}
	return prefix + "." + name
}

func boolString(value bool) string {
	if value {
		return "true"
	}
	return "false"
}

func permissionScope(permission Permission) string {
	if permission.TeamScoped {
		return "team-scoped"
	}
	return "not team-scoped"
}

// NodeSummary describes a node in concise changelog language.
func NodeSummary(node Node) string {
	qualifier := ""
	switch {
	case node.Required:
		qualifier = "required "
	case node.Optional && node.Computed:
		qualifier = "optional, computed "
	case node.Optional:
		qualifier = "optional "
	case node.Computed:
		qualifier = "computed "
	}
	if node.Sensitive {
		qualifier += "sensitive "
	}
	if node.WriteOnly {
		qualifier += "write-only "
	}
	if node.Kind == "block" {
		return qualifier + node.NestingMode + " block"
	}
	if node.NestingMode != "" {
		return qualifier + node.NestingMode + " nested attribute"
	}
	return qualifier + FormatType(node.Type) + " attribute"
}

// FormatType turns Terraform's JSON type encoding into readable text.
func FormatType(raw json.RawMessage) string {
	if len(raw) == 0 {
		return "unspecified"
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return string(raw)
	}
	return formatTypeValue(value)
}

func formatTypeValue(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case []any:
		if len(typed) < 2 {
			return fmt.Sprint(typed)
		}
		kind, _ := typed[0].(string)
		switch kind {
		case "object":
			attributes, ok := typed[1].(map[string]any)
			if !ok {
				return fmt.Sprint(typed)
			}
			names := make([]string, 0, len(attributes))
			for name := range attributes {
				names = append(names, name)
			}
			sort.Strings(names)
			parts := make([]string, 0, len(names))
			for _, name := range names {
				parts = append(parts, name+"="+formatTypeValue(attributes[name]))
			}
			result := "object({" + strings.Join(parts, ", ") + "})"
			if len(typed) > 2 {
				if optional, ok := typed[2].([]any); ok {
					optionalNames := make([]string, 0, len(optional))
					for _, name := range optional {
						optionalNames = append(optionalNames, fmt.Sprint(name))
					}
					result += " optional(" + strings.Join(optionalNames, ", ") + ")"
				}
			}
			return result
		case "tuple":
			elements, ok := typed[1].([]any)
			if !ok {
				return fmt.Sprint(typed)
			}
			parts := make([]string, 0, len(elements))
			for _, element := range elements {
				parts = append(parts, formatTypeValue(element))
			}
			return "tuple([" + strings.Join(parts, ", ") + "])"
		default:
			return fmt.Sprintf("%s(%s)", kind, formatTypeValue(typed[1]))
		}
	default:
		return fmt.Sprint(typed)
	}
}
