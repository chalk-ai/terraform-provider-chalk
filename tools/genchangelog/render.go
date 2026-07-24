package main

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/chalk-ai/terraform-provider-chalk/tools/internal/providerschema"
)

func renderChangelog(live *providerschema.Snapshot, snapshots []versionedSnapshot) ([]byte, error) {
	if len(snapshots) == 0 {
		return nil, fmt.Errorf("cannot render changelog without snapshots")
	}

	var output bytes.Buffer
	output.WriteString(`---
subcategory: ""
page_title: "Chalk Provider: Changelog"
description: |-
  Machine-generated changes to Chalk Terraform resources, data sources, attributes, and required permissions.
---

# Chalk provider changelog

This changelog is generated from the provider's Terraform schemas and required permissions.
For implementation changes and bug fixes, see the [GitHub release notes](https://github.com/chalk-ai/terraform-provider-chalk/releases).

## Unreleased

`)
	writeChanges(&output, providerschema.Diff(snapshots[len(snapshots)-1].value, live))

	for index := len(snapshots) - 1; index >= 1; index-- {
		fmt.Fprintf(&output, "\n## %s\n\n", snapshots[index].value.Version)
		writeChanges(&output, providerschema.Diff(snapshots[index-1].value, snapshots[index].value))
	}

	fmt.Fprintf(&output, "\n## %s\n\n", snapshots[0].value.Version)
	output.WriteString("Baseline snapshot. Schema and permission changes from earlier releases are not included.\n")
	return output.Bytes(), nil
}

func writeChanges(output *bytes.Buffer, changes []providerschema.Change) {
	if len(changes) == 0 {
		output.WriteString("No schema or permission changes.\n")
		return
	}

	resourceChanges := filterChanges(changes, "resource", false)
	dataSourceChanges := filterChanges(changes, "data_source", false)
	permissionChanges := filterPermissionChanges(changes)

	wroteSection := false
	if len(resourceChanges) > 0 {
		writeSectionSeparator(output, wroteSection)
		output.WriteString("### Resources\n\n")
		writeChangeList(output, resourceChanges)
		wroteSection = true
	}
	if len(dataSourceChanges) > 0 {
		writeSectionSeparator(output, wroteSection)
		output.WriteString("### Data sources\n\n")
		writeChangeList(output, dataSourceChanges)
		wroteSection = true
	}
	if len(permissionChanges) > 0 {
		writeSectionSeparator(output, wroteSection)
		output.WriteString("### Required permissions\n\n")
		writeChangeList(output, permissionChanges)
	}
}

func writeSectionSeparator(output *bytes.Buffer, needed bool) {
	if needed {
		output.WriteByte('\n')
	}
}

func filterChanges(changes []providerschema.Change, entityKind string, permissions bool) []providerschema.Change {
	var filtered []providerschema.Change
	for _, change := range changes {
		if change.EntityKind == entityKind && isPermissionChange(change) == permissions {
			filtered = append(filtered, change)
		}
	}
	return filtered
}

func filterPermissionChanges(changes []providerschema.Change) []providerschema.Change {
	var filtered []providerschema.Change
	for _, change := range changes {
		if isPermissionChange(change) {
			filtered = append(filtered, change)
		}
	}
	return filtered
}

func isPermissionChange(change providerschema.Change) bool {
	switch change.Kind {
	case providerschema.ChangePermissionAdded,
		providerschema.ChangePermissionRemoved,
		providerschema.ChangePermissionScope:
		return true
	default:
		return false
	}
}

func writeChangeList(output *bytes.Buffer, changes []providerschema.Change) {
	for _, change := range changes {
		fmt.Fprintf(output, "- %s\n", describeChange(change))
	}
}

func describeChange(change providerschema.Change) string {
	entity := fmt.Sprintf("`%s`", change.Entity)
	path := entity
	if change.Path != "" {
		path = fmt.Sprintf("`%s.%s`", change.Entity, change.Path)
	}
	switch change.Kind {
	case providerschema.ChangeEntityAdded:
		return fmt.Sprintf("Added %s.", entity)
	case providerschema.ChangeEntityRemoved:
		return fmt.Sprintf("Removed %s.", entity)
	case providerschema.ChangeNodeAdded:
		return fmt.Sprintf("Added %s to %s.", change.After, path)
	case providerschema.ChangeNodeRemoved:
		return fmt.Sprintf("Removed %s from %s.", change.Before, path)
	case providerschema.ChangeNodeKind:
		return fmt.Sprintf("Changed %s from %s to %s.", path, change.Before, change.After)
	case providerschema.ChangeType:
		return fmt.Sprintf("Changed %s type from `%s` to `%s`.", path, change.Before, change.After)
	case providerschema.ChangeNesting:
		return fmt.Sprintf("Changed %s nesting from %s to %s.", path, displayEmpty(change.Before), displayEmpty(change.After))
	case providerschema.ChangeOptional:
		return fmt.Sprintf("Changed %s optional from %s to %s.", path, change.Before, change.After)
	case providerschema.ChangeRequired:
		return fmt.Sprintf("Changed %s required from %s to %s.", path, change.Before, change.After)
	case providerschema.ChangeComputed:
		return fmt.Sprintf("Changed %s computed from %s to %s.", path, change.Before, change.After)
	case providerschema.ChangeSensitive:
		return fmt.Sprintf("Changed %s sensitive from %s to %s.", path, change.Before, change.After)
	case providerschema.ChangeWriteOnly:
		return fmt.Sprintf("Changed %s write-only from %s to %s.", path, change.Before, change.After)
	case providerschema.ChangeMinItems:
		return fmt.Sprintf("Changed %s minimum items from %s to %s.", path, change.Before, change.After)
	case providerschema.ChangeMaxItems:
		return fmt.Sprintf("Changed %s maximum items from %s to %s.", path, change.Before, change.After)
	case providerschema.ChangePermissionAdded:
		return fmt.Sprintf("%s now requires `%s`%s.", entityWithKind(change), change.Path, scopeSuffix(change.After))
	case providerschema.ChangePermissionRemoved:
		return fmt.Sprintf("%s no longer requires `%s`%s.", entityWithKind(change), change.Path, scopeSuffix(change.Before))
	case providerschema.ChangePermissionScope:
		return fmt.Sprintf("%s changed `%s` from %s to %s.", entityWithKind(change), change.Path, change.Before, change.After)
	default:
		return fmt.Sprintf("%s changed (%s).", path, change.Kind)
	}
}

func entityWithKind(change providerschema.Change) string {
	if change.EntityKind == "data_source" {
		return fmt.Sprintf("Data source `%s`", change.Entity)
	}
	return fmt.Sprintf("Resource `%s`", change.Entity)
}

func scopeSuffix(scope string) string {
	if scope == "team-scoped" {
		return " (team-scoped)"
	}
	return ""
}

func displayEmpty(value string) string {
	if strings.TrimSpace(value) == "" {
		return "unnested"
	}
	return value
}
