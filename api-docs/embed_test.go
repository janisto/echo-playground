package apidocs

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"

	"sigs.k8s.io/yaml"
)

func TestEmbeddedOpenAPIIsCurrentSemanticVersion(t *testing.T) {
	var document map[string]any
	if err := json.Unmarshal(OpenAPIJSON, &document); err != nil {
		t.Fatalf("decode embedded OpenAPI: %v", err)
	}
	if document["openapi"] != "3.1.2" {
		t.Fatalf("openapi = %v, want 3.1.2", document["openapi"])
	}
	if document["jsonSchemaDialect"] != "https://json-schema.org/draft/2020-12/schema" {
		t.Fatalf("jsonSchemaDialect = %v", document["jsonSchemaDialect"])
	}
}

func TestGeneratedJSONAndYAMLMatch(t *testing.T) {
	yamlData, err := os.ReadFile("swagger.yaml")
	if err != nil {
		t.Fatalf("read YAML: %v", err)
	}
	yamlJSON, err := yaml.YAMLToJSON(yamlData)
	if err != nil {
		t.Fatalf("convert YAML: %v", err)
	}
	var jsonDocument, yamlDocument any
	if err := json.Unmarshal(OpenAPIJSON, &jsonDocument); err != nil {
		t.Fatalf("decode JSON: %v", err)
	}
	if err := json.Unmarshal(yamlJSON, &yamlDocument); err != nil {
		t.Fatalf("decode converted YAML: %v", err)
	}
	if !reflect.DeepEqual(jsonDocument, yamlDocument) {
		t.Fatal("generated JSON and YAML are not semantically equivalent")
	}
}
