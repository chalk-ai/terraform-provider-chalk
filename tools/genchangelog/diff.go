package main

import "sort"

const (
	changeEntityAdded      = "entity_added"
	changeEntityRemoved    = "entity_removed"
	changeAttributeAdded   = "attribute_added"
	changeAttributeRemoved = "attribute_removed"
	changeAttributeType    = "attribute_type"
	changeRequired         = "attribute_required"
	changePermissions      = "permissions"
)

type Change struct {
	EntityKind string
	Entity     string
	Attribute  string
	Kind       string
	Before     string
	After      string
}

func diffSnapshots(old, current Snapshot) []Change {
	var changes []Change
	changes = append(changes, diffEntities("resource", old.Resources, current.Resources)...)
	changes = append(changes, diffEntities("data_source", old.DataSources, current.DataSources)...)
	sort.Slice(changes, func(i, j int) bool {
		left, right := changes[i], changes[j]
		if left.EntityKind != right.EntityKind {
			return left.EntityKind < right.EntityKind
		}
		if left.Entity != right.Entity {
			return left.Entity < right.Entity
		}
		if left.Attribute != right.Attribute {
			return left.Attribute < right.Attribute
		}
		return left.Kind < right.Kind
	})
	return changes
}

func diffEntities(kind string, old, current map[string]Entity) []Change {
	var changes []Change
	for name, oldEntity := range old {
		currentEntity, exists := current[name]
		if !exists {
			changes = append(changes, Change{EntityKind: kind, Entity: name, Kind: changeEntityRemoved})
			continue
		}
		changes = append(changes, diffAttributes(kind, name, oldEntity.Attributes, currentEntity.Attributes)...)
		if oldEntity.Permissions != currentEntity.Permissions {
			changes = append(changes, Change{
				EntityKind: kind,
				Entity:     name,
				Kind:       changePermissions,
				Before:     oldEntity.Permissions,
				After:      currentEntity.Permissions,
			})
		}
	}
	for name := range current {
		if _, exists := old[name]; !exists {
			changes = append(changes, Change{EntityKind: kind, Entity: name, Kind: changeEntityAdded})
		}
	}
	return changes
}

func diffAttributes(kind, entity string, old, current map[string]Attribute) []Change {
	var changes []Change
	for path, oldAttribute := range old {
		currentAttribute, exists := current[path]
		if !exists {
			changes = append(changes, Change{
				EntityKind: kind,
				Entity:     entity,
				Attribute:  path,
				Kind:       changeAttributeRemoved,
				Before:     oldAttribute.Type,
			})
			continue
		}
		if oldAttribute.Type != currentAttribute.Type {
			changes = append(changes, Change{
				EntityKind: kind,
				Entity:     entity,
				Attribute:  path,
				Kind:       changeAttributeType,
				Before:     oldAttribute.Type,
				After:      currentAttribute.Type,
			})
		}
		if oldAttribute.Required != currentAttribute.Required {
			changes = append(changes, Change{
				EntityKind: kind,
				Entity:     entity,
				Attribute:  path,
				Kind:       changeRequired,
				Before:     requiredString(oldAttribute.Required),
				After:      requiredString(currentAttribute.Required),
			})
		}
	}
	for path, attribute := range current {
		if _, exists := old[path]; !exists {
			changes = append(changes, Change{
				EntityKind: kind,
				Entity:     entity,
				Attribute:  path,
				Kind:       changeAttributeAdded,
				After:      attribute.Type,
			})
		}
	}
	return changes
}

func requiredString(required bool) string {
	if required {
		return "required"
	}
	return "not required"
}
