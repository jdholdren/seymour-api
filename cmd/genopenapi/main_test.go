package main

import (
	"encoding/json"
	"testing"

	"github.com/swaggest/openapi-go/openapi3"
)

func TestApplyFilterSchemaContract(t *testing.T) {
	// The reflected fixture mirrors the optional pointer and nullable slice
	// schemas generated for filter requests, plus an unrelated nullable array.
	spec := &openapi3.Spec{}
	fixture := map[string]any{
		"openapi": "3.0.0",
		"info":    map[string]any{"title": "test", "version": "test"},
		"paths":   map[string]any{},
		"components": map[string]any{"schemas": map[string]any{
			"V1PostFilterReq": requestSchema(true),
			"V1PutFilterReq":  requestSchema(false),
			"V1FilterResp":    responseSchema(),
			"V1FilterListResp": map[string]any{"properties": map[string]any{
				"filters": map[string]any{"type": "array", "nullable": true},
			}},
			"V1FilterListConfig": map[string]any{"properties": map[string]any{
				"allowlist":    map[string]any{"type": "array", "nullable": true},
				"disallowlist": map[string]any{"type": "array", "nullable": true},
			}},
			"V1FilterAIConfig": map[string]any{"type": "object"},
			"UnrelatedList": map[string]any{"properties": map[string]any{
				"items": map[string]any{"type": "array", "nullable": true},
			}},
		}},
	}
	encoded, err := json.Marshal(fixture)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(encoded, spec); err != nil {
		t.Fatal(err)
	}

	// The contract transformation tightens the filter variants without
	// changing reflected input nullability or unrelated schemas.
	if err := applyFilterSchemaContract(spec); err != nil {
		t.Fatal(err)
	}
	encoded, err = json.Marshal(spec)
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if err := json.Unmarshal(encoded, &document); err != nil {
		t.Fatal(err)
	}
	schemas := document["components"].(map[string]any)["schemas"].(map[string]any)
	assertOneOfTypes(t, schemas, "V1PostFilterReq", "type", "ai", "list")
	assertOneOfTypes(t, schemas, "V1PutFilterReq", "", "ai_config", "list_config")
	assertOneOfTypes(t, schemas, "V1FilterResp", "type", "ai", "list")

	postVariants := schemaOneOf(t, schemas, "V1PostFilterReq")
	for _, variant := range postVariants {
		props := variant["properties"].(map[string]any)
		if _, exists := props["enabled"].(map[string]any)["nullable"]; exists {
			t.Fatal("enabled should not be nullable")
		}
	}
	listConfig := schemas["V1FilterListConfig"].(map[string]any)["properties"].(map[string]any)
	if listConfig["allowlist"].(map[string]any)["nullable"] != true {
		t.Fatal("request allowlist nullability should remain unchanged")
	}
	filters := schemas["V1FilterListResp"].(map[string]any)["properties"].(map[string]any)["filters"].(map[string]any)
	if _, exists := filters["nullable"]; exists {
		t.Fatal("filter response array should be non-null")
	}
	listRespVariant := schemaOneOf(t, schemas, "V1FilterResp")[1]
	responseLists := listRespVariant["properties"].(map[string]any)["list_config"].(map[string]any)["properties"].(map[string]any)
	if _, exists := responseLists["allowlist"].(map[string]any)["nullable"]; exists {
		t.Fatal("response allowlist should be non-null")
	}
	unrelated := schemas["UnrelatedList"].(map[string]any)["properties"].(map[string]any)["items"].(map[string]any)
	if unrelated["nullable"] != true {
		t.Fatal("unrelated nullable array should remain unchanged")
	}
}

func requestSchema(post bool) map[string]any {
	props := map[string]any{
		"enabled":     map[string]any{"type": "boolean", "nullable": true},
		"ai_config":   map[string]any{"$ref": "#/components/schemas/V1FilterAIConfig"},
		"list_config": map[string]any{"$ref": "#/components/schemas/V1FilterListConfig"},
	}
	if post {
		props["type"] = map[string]any{"type": "string", "enum": []any{"ai", "list"}}
	}
	return map[string]any{"type": "object", "properties": props, "required": []any{"enabled"}}
}

func responseSchema() map[string]any {
	return map[string]any{"type": "object", "properties": map[string]any{
		"id":          map[string]any{"type": "string"},
		"user_id":     map[string]any{"type": "string"},
		"type":        map[string]any{"type": "string"},
		"enabled":     map[string]any{"type": "boolean"},
		"ai_config":   map[string]any{"$ref": "#/components/schemas/V1FilterAIConfig"},
		"list_config": map[string]any{"$ref": "#/components/schemas/V1FilterListConfig"},
		"created_at":  map[string]any{"type": "string", "format": "date-time"},
		"updated_at":  map[string]any{"type": "string", "format": "date-time"},
	}}
}

func assertOneOfTypes(t *testing.T, schemas map[string]any, schemaName, discriminator string, expected ...string) {
	t.Helper()
	variants := schemaOneOf(t, schemas, schemaName)
	if len(variants) != len(expected) {
		t.Fatalf("%s has %d variants, want %d", schemaName, len(variants), len(expected))
	}
	for i, variant := range variants {
		props := variant["properties"].(map[string]any)
		if discriminator == "" {
			if _, ok := props[expected[i]].(map[string]any); !ok {
				t.Errorf("%s variant %d missing config %s", schemaName, i, expected[i])
			}
			continue
		}
		discriminatorSchema := props[discriminator].(map[string]any)
		enum := discriminatorSchema["enum"].([]any)
		if len(enum) != 1 || enum[0] != expected[i] {
			t.Errorf("%s variant %d discriminator enum = %v, want [%s]", schemaName, i, enum, expected[i])
		}
	}
}

func schemaOneOf(t *testing.T, schemas map[string]any, name string) []map[string]any {
	t.Helper()
	schema := schemas[name].(map[string]any)
	values := schema["oneOf"].([]any)
	variants := make([]map[string]any, len(values))
	for i, value := range values {
		variants[i] = value.(map[string]any)
	}
	return variants
}
