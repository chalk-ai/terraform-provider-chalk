package main

import (
	"bytes"
	"fmt"
	"strings"
)

func renderChangelog(live Snapshot, snapshots []Snapshot) []byte {
	var output bytes.Buffer
	output.WriteString(`---
subcategory: ""
page_title: "Chalk Provider: Changelog"
description: |-
  Changes to Chalk Terraform resources, data sources, attributes, and required permissions.
---

# Chalk provider changelog

This file is generated from provider schema snapshots.

## Unreleased

`)
	writeChanges(&output, diffSnapshots(snapshots[len(snapshots)-1], live))

	for index := len(snapshots) - 1; index >= 1; index-- {
		fmt.Fprintf(&output, "\n## %s\n\n", snapshots[index].Version)
		writeChanges(&output, diffSnapshots(snapshots[index-1], snapshots[index]))
	}

	fmt.Fprintf(&output, "\n## %s\n\n", snapshots[0].Version)
	output.WriteString("Baseline snapshot.\n")
	return output.Bytes()
}

func writeChanges(output *bytes.Buffer, changes []Change) {
	if len(changes) == 0 {
		output.WriteString("No schema or permission changes.\n")
		return
	}

	sections := []struct {
		title string
		match func(Change) bool
	}{
		{"Resources", func(change Change) bool {
			return change.EntityKind == "resource" && change.Kind != changePermissions
		}},
		{"Data sources", func(change Change) bool {
			return change.EntityKind == "data_source" && change.Kind != changePermissions
		}},
		{"Required permissions", func(change Change) bool {
			return change.Kind == changePermissions
		}},
	}

	wroteSection := false
	for _, section := range sections {
		var matching []Change
		for _, change := range changes {
			if section.match(change) {
				matching = append(matching, change)
			}
		}
		if len(matching) == 0 {
			continue
		}
		if wroteSection {
			output.WriteByte('\n')
		}
		fmt.Fprintf(output, "### %s\n\n", section.title)
		for _, change := range matching {
			fmt.Fprintf(output, "- %s\n", describeChange(change))
		}
		wroteSection = true
	}
}

func describeChange(change Change) string {
	entity := fmt.Sprintf("`%s`", change.Entity)
	attribute := fmt.Sprintf("`%s.%s`", change.Entity, change.Attribute)
	switch change.Kind {
	case changeEntityAdded:
		return fmt.Sprintf("Added %s.", entity)
	case changeEntityRemoved:
		return fmt.Sprintf("Removed %s.", entity)
	case changeAttributeAdded:
		return fmt.Sprintf("Added attribute %s (`%s`).", attribute, change.After)
	case changeAttributeRemoved:
		return fmt.Sprintf("Removed attribute %s (`%s`).", attribute, change.Before)
	case changeAttributeType:
		return fmt.Sprintf("Changed attribute %s type from `%s` to `%s`.", attribute, change.Before, change.After)
	case changeRequired:
		return fmt.Sprintf("Changed attribute %s from %s to %s.", attribute, change.Before, change.After)
	case changePermissions:
		return fmt.Sprintf("%s permissions changed from %s to %s.", entity, permissionText(change.Before), permissionText(change.After))
	default:
		return fmt.Sprintf("%s changed.", entity)
	}
}

func permissionText(markdown string) string {
	const marker = "**Required permissions:**"
	value := strings.TrimSpace(strings.TrimPrefix(markdown, marker))
	if value == "" {
		return "none"
	}
	return value
}
