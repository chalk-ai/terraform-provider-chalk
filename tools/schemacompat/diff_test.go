package main

import (
	"encoding/json"
	"testing"
)

// --- helpers ---

func makeSchema(providers map[string]*ProviderSchema) *ProviderSchemaOutput {
	return &ProviderSchemaOutput{
		FormatVersion:   "1.0",
		ProviderSchemas: providers,
	}
}

func makeProvider(resources, dataSources map[string]*SchemaEntry) *ProviderSchema {
	return &ProviderSchema{
		ResourceSchemas:   resources,
		DataSourceSchemas: dataSources,
	}
}

func makeEntry(attrs map[string]*Attribute, blocks map[string]*BlockType) *SchemaEntry {
	return &SchemaEntry{
		Block: Block{
			Attributes: attrs,
			BlockTypes: blocks,
		},
	}
}

func makeBlock(attrs map[string]*Attribute, blocks map[string]*BlockType) BlockType {
	return BlockType{
		Block: Block{
			Attributes: attrs,
			BlockTypes: blocks,
		},
	}
}

// jsonStr returns a json.RawMessage for a JSON string type like "string".
func jsonStr(s string) json.RawMessage {
	b, _ := json.Marshal(s)
	return b
}

// jsonRaw returns a json.RawMessage from a raw JSON literal.
func jsonRaw(s string) json.RawMessage {
	return json.RawMessage(s)
}

const testProvider = "registry.terraform.io/chalk-ai/chalk"

func singleProvider(resources, dataSources map[string]*SchemaEntry) *ProviderSchemaOutput {
	return makeSchema(map[string]*ProviderSchema{
		testProvider: makeProvider(resources, dataSources),
	})
}

func assertNoChanges(t *testing.T, old, new *ProviderSchemaOutput) {
	t.Helper()
	changes := Diff(old, new)
	if len(changes) != 0 {
		t.Errorf("expected no breaking changes, got %d: %v", len(changes), changes)
	}
}

func assertBreaking(t *testing.T, old, new *ProviderSchemaOutput, wantRules ...string) {
	t.Helper()
	changes := Diff(old, new)
	got := make(map[string]bool)
	for _, c := range changes {
		got[c.Rule] = true
	}
	for _, rule := range wantRules {
		if !got[rule] {
			t.Errorf("expected breaking change rule %s, got changes: %v", rule, changes)
		}
	}
}

func assertNotBreaking(t *testing.T, old, new *ProviderSchemaOutput, rule string) {
	t.Helper()
	changes := Diff(old, new)
	for _, c := range changes {
		if c.Rule == rule {
			t.Errorf("expected no %s breaking change, but got: %s", rule, c)
		}
	}
}

// --- R001: resource deleted ---

func TestDiff_R001_ResourceDeleted(t *testing.T) {
	t.Parallel()
	old := singleProvider(map[string]*SchemaEntry{"chalk_foo": makeEntry(nil, nil)}, nil)
	new := singleProvider(nil, nil)
	assertBreaking(t, old, new, "R001")
}

func TestDiff_R001_ResourceAdded_NotBreaking(t *testing.T) {
	t.Parallel()
	old := singleProvider(nil, nil)
	new := singleProvider(map[string]*SchemaEntry{"chalk_foo": makeEntry(nil, nil)}, nil)
	assertNotBreaking(t, old, new, "R001")
}

// --- R002: data source deleted ---

func TestDiff_R002_DataSourceDeleted(t *testing.T) {
	t.Parallel()
	old := singleProvider(nil, map[string]*SchemaEntry{"chalk_foo": makeEntry(nil, nil)})
	new := singleProvider(nil, nil)
	assertBreaking(t, old, new, "R002")
}

func TestDiff_R002_DataSourceAdded_NotBreaking(t *testing.T) {
	t.Parallel()
	old := singleProvider(nil, nil)
	new := singleProvider(nil, map[string]*SchemaEntry{"chalk_foo": makeEntry(nil, nil)})
	assertNotBreaking(t, old, new, "R002")
}

// --- R003: attribute deleted ---

func TestDiff_R003_AttributeDeleted(t *testing.T) {
	t.Parallel()
	old := singleProvider(map[string]*SchemaEntry{
		"chalk_foo": makeEntry(map[string]*Attribute{
			"name": {Type: jsonStr("string"), Optional: true},
		}, nil),
	}, nil)
	new := singleProvider(map[string]*SchemaEntry{
		"chalk_foo": makeEntry(nil, nil),
	}, nil)
	assertBreaking(t, old, new, "R003")
}

func TestDiff_R003_AttributeAdded_Optional_NotBreaking(t *testing.T) {
	t.Parallel()
	old := singleProvider(map[string]*SchemaEntry{"chalk_foo": makeEntry(nil, nil)}, nil)
	new := singleProvider(map[string]*SchemaEntry{
		"chalk_foo": makeEntry(map[string]*Attribute{
			"name": {Type: jsonStr("string"), Optional: true},
		}, nil),
	}, nil)
	assertNotBreaking(t, old, new, "R003")
}

// --- R004: block deleted ---

func TestDiff_R004_BlockDeleted(t *testing.T) {
	t.Parallel()
	blk := makeBlock(map[string]*Attribute{"key": {Type: jsonStr("string"), Optional: true}}, nil)
	old := singleProvider(map[string]*SchemaEntry{
		"chalk_foo": makeEntry(nil, map[string]*BlockType{"config": &blk}),
	}, nil)
	new := singleProvider(map[string]*SchemaEntry{
		"chalk_foo": makeEntry(nil, nil),
	}, nil)
	assertBreaking(t, old, new, "R004")
}

func TestDiff_R004_BlockAdded_Optional_NotBreaking(t *testing.T) {
	t.Parallel()
	blk := makeBlock(nil, nil) // min_items=0, optional
	old := singleProvider(map[string]*SchemaEntry{"chalk_foo": makeEntry(nil, nil)}, nil)
	new := singleProvider(map[string]*SchemaEntry{
		"chalk_foo": makeEntry(nil, map[string]*BlockType{"config": &blk}),
	}, nil)
	assertNotBreaking(t, old, new, "R004")
}

// --- R005: attribute type changed ---

func TestDiff_R005_TypeChanged(t *testing.T) {
	t.Parallel()
	old := singleProvider(map[string]*SchemaEntry{
		"chalk_foo": makeEntry(map[string]*Attribute{
			"count": {Type: jsonStr("string"), Optional: true},
		}, nil),
	}, nil)
	new := singleProvider(map[string]*SchemaEntry{
		"chalk_foo": makeEntry(map[string]*Attribute{
			"count": {Type: jsonStr("number"), Optional: true},
		}, nil),
	}, nil)
	assertBreaking(t, old, new, "R005")
}

func TestDiff_R005_ComplexTypeChanged(t *testing.T) {
	t.Parallel()
	old := singleProvider(map[string]*SchemaEntry{
		"chalk_foo": makeEntry(map[string]*Attribute{
			"tags": {Type: jsonRaw(`["list","string"]`), Optional: true},
		}, nil),
	}, nil)
	new := singleProvider(map[string]*SchemaEntry{
		"chalk_foo": makeEntry(map[string]*Attribute{
			"tags": {Type: jsonRaw(`["set","string"]`), Optional: true},
		}, nil),
	}, nil)
	assertBreaking(t, old, new, "R005")
}

func TestDiff_R005_TypeUnchanged_NotBreaking(t *testing.T) {
	t.Parallel()
	entry := makeEntry(map[string]*Attribute{
		"name": {Type: jsonStr("string"), Optional: true},
	}, nil)
	schema := singleProvider(map[string]*SchemaEntry{"chalk_foo": entry}, nil)
	assertNoChanges(t, schema, schema)
}

// --- R006: optional attribute → required ---

func TestDiff_R006_OptionalToRequired(t *testing.T) {
	t.Parallel()
	old := singleProvider(map[string]*SchemaEntry{
		"chalk_foo": makeEntry(map[string]*Attribute{
			"name": {Type: jsonStr("string"), Optional: true},
		}, nil),
	}, nil)
	new := singleProvider(map[string]*SchemaEntry{
		"chalk_foo": makeEntry(map[string]*Attribute{
			"name": {Type: jsonStr("string"), Required: true},
		}, nil),
	}, nil)
	assertBreaking(t, old, new, "R006")
}

func TestDiff_R006_ComputedOnlyToRequired(t *testing.T) {
	t.Parallel()
	old := singleProvider(map[string]*SchemaEntry{
		"chalk_foo": makeEntry(map[string]*Attribute{
			"id": {Type: jsonStr("string"), Computed: true},
		}, nil),
	}, nil)
	cur := singleProvider(map[string]*SchemaEntry{
		"chalk_foo": makeEntry(map[string]*Attribute{
			"id": {Type: jsonStr("string"), Required: true},
		}, nil),
	}, nil)
	assertBreaking(t, old, cur, "R006")
}

func TestDiff_R006_OptionalComputedToRequired(t *testing.T) {
	t.Parallel()
	old := singleProvider(map[string]*SchemaEntry{
		"chalk_foo": makeEntry(map[string]*Attribute{
			"region": {Type: jsonStr("string"), Optional: true, Computed: true},
		}, nil),
	}, nil)
	cur := singleProvider(map[string]*SchemaEntry{
		"chalk_foo": makeEntry(map[string]*Attribute{
			"region": {Type: jsonStr("string"), Required: true},
		}, nil),
	}, nil)
	assertBreaking(t, old, cur, "R006")
}

func TestDiff_R006_RequiredStaysRequired_NotBreaking(t *testing.T) {
	t.Parallel()
	entry := makeEntry(map[string]*Attribute{
		"name": {Type: jsonStr("string"), Required: true},
	}, nil)
	schema := singleProvider(map[string]*SchemaEntry{"chalk_foo": entry}, nil)
	assertNotBreaking(t, schema, schema, "R006")
}

// --- R007: optional block → required ---

func TestDiff_R007_BlockMadeRequired(t *testing.T) {
	t.Parallel()
	optBlock := BlockType{MinItems: 0, Block: Block{}}
	reqBlock := BlockType{MinItems: 1, Block: Block{}}
	old := singleProvider(map[string]*SchemaEntry{
		"chalk_foo": makeEntry(nil, map[string]*BlockType{"config": &optBlock}),
	}, nil)
	new := singleProvider(map[string]*SchemaEntry{
		"chalk_foo": makeEntry(nil, map[string]*BlockType{"config": &reqBlock}),
	}, nil)
	assertBreaking(t, old, new, "R007")
}

func TestDiff_R007_BlockStaysOptional_NotBreaking(t *testing.T) {
	t.Parallel()
	blk := BlockType{MinItems: 0, Block: Block{}}
	schema := singleProvider(map[string]*SchemaEntry{
		"chalk_foo": makeEntry(nil, map[string]*BlockType{"config": &blk}),
	}, nil)
	assertNotBreaking(t, schema, schema, "R007")
}

// --- R008: new required attribute added ---

func TestDiff_R008_NewRequiredAttribute(t *testing.T) {
	t.Parallel()
	old := singleProvider(map[string]*SchemaEntry{"chalk_foo": makeEntry(nil, nil)}, nil)
	new := singleProvider(map[string]*SchemaEntry{
		"chalk_foo": makeEntry(map[string]*Attribute{
			"region": {Type: jsonStr("string"), Required: true},
		}, nil),
	}, nil)
	assertBreaking(t, old, new, "R008")
}

func TestDiff_R008_NewOptionalAttribute_NotBreaking(t *testing.T) {
	t.Parallel()
	old := singleProvider(map[string]*SchemaEntry{"chalk_foo": makeEntry(nil, nil)}, nil)
	new := singleProvider(map[string]*SchemaEntry{
		"chalk_foo": makeEntry(map[string]*Attribute{
			"region": {Type: jsonStr("string"), Optional: true},
		}, nil),
	}, nil)
	assertNotBreaking(t, old, new, "R008")
}

// --- R009: new required block added ---

func TestDiff_R009_NewRequiredBlock(t *testing.T) {
	t.Parallel()
	reqBlock := BlockType{MinItems: 1, Block: Block{}}
	old := singleProvider(map[string]*SchemaEntry{"chalk_foo": makeEntry(nil, nil)}, nil)
	new := singleProvider(map[string]*SchemaEntry{
		"chalk_foo": makeEntry(nil, map[string]*BlockType{"config": &reqBlock}),
	}, nil)
	assertBreaking(t, old, new, "R009")
}

func TestDiff_R009_NewOptionalBlock_NotBreaking(t *testing.T) {
	t.Parallel()
	optBlock := BlockType{MinItems: 0, Block: Block{}}
	old := singleProvider(map[string]*SchemaEntry{"chalk_foo": makeEntry(nil, nil)}, nil)
	new := singleProvider(map[string]*SchemaEntry{
		"chalk_foo": makeEntry(nil, map[string]*BlockType{"config": &optBlock}),
	}, nil)
	assertNotBreaking(t, old, new, "R009")
}

// --- nested block changes ---

func TestDiff_NestedAttributeDeleted(t *testing.T) {
	t.Parallel()
	oldBlock := BlockType{
		Block: Block{
			Attributes: map[string]*Attribute{
				"key": {Type: jsonStr("string"), Optional: true},
			},
		},
	}
	newBlock := BlockType{Block: Block{}}
	old := singleProvider(map[string]*SchemaEntry{
		"chalk_foo": makeEntry(nil, map[string]*BlockType{"config": &oldBlock}),
	}, nil)
	new := singleProvider(map[string]*SchemaEntry{
		"chalk_foo": makeEntry(nil, map[string]*BlockType{"config": &newBlock}),
	}, nil)
	assertBreaking(t, old, new, "R003")
}

func TestDiff_NestedRequiredAttributeAdded(t *testing.T) {
	t.Parallel()
	oldBlock := BlockType{Block: Block{}}
	newBlock := BlockType{
		Block: Block{
			Attributes: map[string]*Attribute{
				"key": {Type: jsonStr("string"), Required: true},
			},
		},
	}
	old := singleProvider(map[string]*SchemaEntry{
		"chalk_foo": makeEntry(nil, map[string]*BlockType{"config": &oldBlock}),
	}, nil)
	new := singleProvider(map[string]*SchemaEntry{
		"chalk_foo": makeEntry(nil, map[string]*BlockType{"config": &newBlock}),
	}, nil)
	assertBreaking(t, old, new, "R008")
}

// --- safe changes ---

func TestDiff_SafeChanges_NotBreaking(t *testing.T) {
	t.Parallel()
	// Adding optional attrs, adding optional blocks, adding new resources — all safe.
	old := singleProvider(map[string]*SchemaEntry{
		"chalk_foo": makeEntry(map[string]*Attribute{
			"name": {Type: jsonStr("string"), Required: true},
		}, nil),
	}, nil)
	new := singleProvider(map[string]*SchemaEntry{
		"chalk_foo": makeEntry(map[string]*Attribute{
			"name":        {Type: jsonStr("string"), Required: true},
			"description": {Type: jsonStr("string"), Optional: true},
		}, nil),
		"chalk_bar": makeEntry(nil, nil),
	}, nil)
	assertNoChanges(t, old, new)
}
