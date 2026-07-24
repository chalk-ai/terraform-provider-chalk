package providerschema

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
)

const FormatVersion = 1

var releaseVersionPattern = regexp.MustCompile(`^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)

// Snapshot is the stable, versioned representation of the provider contract
// used to generate the Terraform Registry changelog.
type Snapshot struct {
	FormatVersion int               `json:"format_version"`
	Version       string            `json:"version"`
	Resources     map[string]Entity `json:"resources"`
	DataSources   map[string]Entity `json:"data_sources"`
}

// Entity is a Terraform resource or data source.
type Entity struct {
	Schema      map[string]Node `json:"schema,omitempty"`
	Permissions []Permission    `json:"permissions,omitempty"`
}

// Node is an attribute or block at one level of an entity's schema.
type Node struct {
	Kind        string          `json:"kind"`
	Type        json.RawMessage `json:"type,omitempty"`
	NestingMode string          `json:"nesting_mode,omitempty"`
	Optional    bool            `json:"optional,omitempty"`
	Required    bool            `json:"required,omitempty"`
	Computed    bool            `json:"computed,omitempty"`
	Sensitive   bool            `json:"sensitive,omitempty"`
	WriteOnly   bool            `json:"write_only,omitempty"`
	MinItems    int             `json:"min_items,omitempty"`
	MaxItems    int             `json:"max_items,omitempty"`
	Children    map[string]Node `json:"children,omitempty"`
}

// Permission is a Chalk permission required by a resource or data source.
type Permission struct {
	Name       string `json:"name"`
	TeamScoped bool   `json:"team_scoped,omitempty"`
}

// NewSnapshot returns an initialized snapshot.
func NewSnapshot(version string) *Snapshot {
	return &Snapshot{
		FormatVersion: FormatVersion,
		Version:       version,
		Resources:     map[string]Entity{},
		DataSources:   map[string]Entity{},
	}
}

// ValidateReleaseVersion accepts the repository's release-tag format.
func ValidateReleaseVersion(version string) error {
	if !releaseVersionPattern.MatchString(version) {
		return fmt.Errorf("version %q must match vMAJOR.MINOR.PATCH", version)
	}
	return nil
}

// LoadSnapshot reads and validates a snapshot.
func LoadSnapshot(path string) (*Snapshot, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading snapshot: %w", err)
	}
	var snapshot Snapshot
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return nil, fmt.Errorf("parsing snapshot: %w", err)
	}
	if err := snapshot.Validate(); err != nil {
		return nil, fmt.Errorf("validating snapshot: %w", err)
	}
	return &snapshot, nil
}

// MarshalSnapshot returns canonical, newline-terminated snapshot JSON.
func MarshalSnapshot(snapshot *Snapshot) ([]byte, error) {
	if err := snapshot.Validate(); err != nil {
		return nil, err
	}
	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshalling snapshot: %w", err)
	}
	return append(data, '\n'), nil
}

// Validate checks the stable snapshot envelope and normalizes nil maps.
func (s *Snapshot) Validate() error {
	if s == nil {
		return errors.New("snapshot is nil")
	}
	if s.FormatVersion != FormatVersion {
		return fmt.Errorf("unsupported format_version %d", s.FormatVersion)
	}
	if err := ValidateReleaseVersion(s.Version); err != nil {
		return err
	}
	if s.Resources == nil {
		s.Resources = map[string]Entity{}
	}
	if s.DataSources == nil {
		s.DataSources = map[string]Entity{}
	}
	if err := validateEntities("resource", s.Resources); err != nil {
		return err
	}
	if err := validateEntities("data source", s.DataSources); err != nil {
		return err
	}
	return nil
}

func validateEntities(kind string, entities map[string]Entity) error {
	for name, entity := range entities {
		if name == "" {
			return fmt.Errorf("%s has an empty name", kind)
		}
		if err := validateNodes(name, entity.Schema); err != nil {
			return err
		}
		for index, permission := range entity.Permissions {
			if permission.Name == "" {
				return fmt.Errorf("%s %s has an empty permission", kind, name)
			}
			if index == 0 {
				continue
			}
			previous := entity.Permissions[index-1]
			if previous.Name >= permission.Name {
				return fmt.Errorf("%s %s permissions are not uniquely sorted", kind, name)
			}
		}
	}
	return nil
}

func validateNodes(prefix string, nodes map[string]Node) error {
	for name, node := range nodes {
		path := joinPath(prefix, name)
		if name == "" {
			return fmt.Errorf("%s has an empty schema name", prefix)
		}
		if node.Required && (node.Optional || node.Computed) {
			return fmt.Errorf("%s is required and optional or computed", path)
		}
		switch node.Kind {
		case "attribute":
			if node.NestingMode == "" && len(node.Type) == 0 {
				return fmt.Errorf("%s leaf attribute has no type", path)
			}
			if node.NestingMode != "" && len(node.Type) != 0 {
				return fmt.Errorf("%s nested attribute also has a leaf type", path)
			}
			if node.NestingMode != "" && !validNestingMode(node.NestingMode, true) {
				return fmt.Errorf("%s has invalid attribute nesting mode %q", path, node.NestingMode)
			}
			if len(node.Type) > 0 {
				if _, err := canonicalType(node.Type); err != nil {
					return fmt.Errorf("%s: %w", path, err)
				}
			}
			if node.MinItems != 0 || node.MaxItems != 0 {
				return fmt.Errorf("%s attribute has block item limits", path)
			}
		case "block":
			if !validNestingMode(node.NestingMode, false) {
				return fmt.Errorf("%s has invalid block nesting mode %q", path, node.NestingMode)
			}
			if len(node.Type) != 0 || node.Optional || node.Required || node.Computed || node.Sensitive || node.WriteOnly {
				return fmt.Errorf("%s block has attribute-only fields", path)
			}
			if node.MinItems < 0 || node.MaxItems < 0 ||
				(node.MaxItems != 0 && node.MinItems > node.MaxItems) {
				return fmt.Errorf("%s has invalid block item limits", path)
			}
		default:
			return fmt.Errorf("%s has invalid node kind %q", path, node.Kind)
		}
		if err := validateNodes(path, node.Children); err != nil {
			return err
		}
	}
	return nil
}

func validNestingMode(mode string, attribute bool) bool {
	switch mode {
	case "single", "list", "set":
		return true
	case "map":
		return true
	case "group":
		return !attribute
	default:
		return false
	}
}

// ContractEqual compares schema and permission content while ignoring release
// metadata such as Version.
func ContractEqual(a, b *Snapshot) bool {
	if a == nil || b == nil {
		return a == b
	}
	return entitiesEqual(a.Resources, b.Resources) &&
		entitiesEqual(a.DataSources, b.DataSources)
}

func entitiesEqual(a, b map[string]Entity) bool {
	left, err := json.Marshal(a)
	if err != nil {
		return false
	}
	right, err := json.Marshal(b)
	if err != nil {
		return false
	}
	return bytes.Equal(left, right)
}

// ParsePermissions parses the controlled Markdown emitted by genpermissions.
func ParsePermissions(markdown string) ([]Permission, error) {
	const marker = "**Required permissions:**"
	index := strings.LastIndex(markdown, marker)
	if index < 0 {
		return nil, nil
	}
	text := strings.TrimSpace(markdown[index+len(marker):])
	if text == "" {
		return nil, errors.New("required-permissions marker has no permissions")
	}

	var permissions []Permission
	for _, part := range strings.Split(text, ",") {
		part = strings.TrimSpace(part)
		if !strings.HasPrefix(part, "`") {
			return nil, fmt.Errorf("invalid permission entry %q", part)
		}
		end := strings.Index(part[1:], "`")
		if end < 0 {
			return nil, fmt.Errorf("invalid permission entry %q", part)
		}
		end++
		name := part[1:end]
		suffix := strings.TrimSpace(part[end+1:])
		teamScoped := false
		switch suffix {
		case "":
		case "*(team-scoped)*":
			teamScoped = true
		default:
			return nil, fmt.Errorf("invalid permission scope %q", suffix)
		}
		permissions = append(permissions, Permission{Name: name, TeamScoped: teamScoped})
	}
	sort.Slice(permissions, func(i, j int) bool {
		if permissions[i].Name != permissions[j].Name {
			return permissions[i].Name < permissions[j].Name
		}
		return !permissions[i].TeamScoped && permissions[j].TeamScoped
	})
	return permissions, nil
}

func canonicalType(raw json.RawMessage) (json.RawMessage, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, fmt.Errorf("invalid Terraform type JSON: %w", err)
	}
	canonical, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("normalizing Terraform type JSON: %w", err)
	}
	return json.RawMessage(canonical), nil
}
