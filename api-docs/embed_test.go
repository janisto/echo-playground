package apidocs

import (
	"crypto/sha256"
	"encoding/json"
	"os"
	"reflect"
	"slices"
	"strconv"
	"testing"

	"sigs.k8s.io/yaml"
)

func TestGeneratedOpenAPIContract(t *testing.T) {
	var document map[string]any
	if err := json.Unmarshal(OpenAPIJSON, &document); err != nil {
		t.Fatalf("decode embedded OpenAPI: %v", err)
	}
	if document["openapi"] != "3.1.0" {
		t.Fatalf("expected OpenAPI 3.1.0, got %v", document["openapi"])
	}

	components := requireDocumentObject(t, document, "components")
	schemes := requireObject(t, components["securitySchemes"])
	bearer := requireObject(t, schemes["BearerAuth"])
	if bearer["type"] != "http" || bearer["scheme"] != "bearer" {
		t.Fatalf("unexpected bearer scheme: %#v", bearer)
	}

	paths := requireObject(t, document["paths"])
	hello := requireObject(t, paths["/hello"])
	post := requireObject(t, hello["post"])
	responses := requireObject(t, post["responses"])
	if _, ok := responses["200"]; !ok {
		t.Fatalf("POST /hello does not document 200: %#v", responses)
	}
	badRequest := requireObject(t, responses["400"])
	content := requireObject(t, badRequest["content"])
	if _, ok := content["application/problem+json"]; !ok {
		t.Fatalf("400 response lacks application/problem+json: %#v", content)
	}
	for _, status := range []string{"413", "415"} {
		if _, ok := responses[status]; !ok {
			t.Fatalf("POST /hello does not document %s: %#v", status, responses)
		}
	}

	profile := requireObject(t, paths["/profile"])
	for _, method := range []string{"get", "post", "patch", "delete"} {
		operation := requireObject(t, profile[method])
		operationResponses := requireObject(t, operation["responses"])
		if _, ok := operationResponses["503"]; !ok {
			t.Fatalf("%s /profile does not document 503: %#v", method, operationResponses)
		}
	}
	assertProblemMediaTypes(t, paths)
}

func assertProblemMediaTypes(t *testing.T, paths map[string]any) {
	t.Helper()
	for pathName, pathValue := range paths {
		path := requireObject(t, pathValue)
		for method, operationValue := range path {
			operation, ok := operationValue.(map[string]any)
			if !ok {
				continue
			}
			responses := requireObject(t, operation["responses"])
			for status, responseValue := range responses {
				statusCode, err := strconv.Atoi(status)
				if err != nil || statusCode < 400 {
					continue
				}
				response := requireObject(t, responseValue)
				content := requireObject(t, response["content"])
				for _, mediaType := range []string{"application/problem+json", "application/problem+cbor"} {
					if _, ok := content[mediaType]; !ok {
						t.Fatalf("%s %s %s lacks %s: %#v", method, pathName, status, mediaType, content)
					}
				}
			}
		}
	}
}

func requireDocumentObject(t *testing.T, document map[string]any, key string) map[string]any {
	t.Helper()
	value, ok := document[key].(map[string]any)
	if ok {
		return value
	}
	keys := make([]string, 0, len(document))
	for documentKey := range document {
		keys = append(keys, documentKey)
	}
	slices.Sort(keys)
	t.Fatalf(
		"expected %q object, got %T; embedded bytes=%d sha256=%x keys=%v",
		key,
		document[key],
		len(OpenAPIJSON),
		sha256.Sum256(OpenAPIJSON),
		keys,
	)
	return nil
}

func requireObject(t *testing.T, value any) map[string]any {
	t.Helper()
	object, ok := value.(map[string]any)
	if !ok {
		t.Fatalf("expected object, got %T", value)
	}
	return object
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
