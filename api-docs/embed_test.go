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
	expected := map[string]map[string]operationContract{
		"/hello": {
			"get": {id: "getHello", statuses: []string{"200", "406"}, successStatus: "200"},
			"post": {
				id:            "createHello",
				statuses:      []string{"200", "400", "406", "413", "415", "422"},
				requestBody:   true,
				successStatus: "200",
			},
		},
		"/items": {
			"get": {
				id:            "listItems",
				statuses:      []string{"200", "400", "406", "422"},
				successStatus: "200",
				header:        "Link",
			},
		},
		"/profile": {
			"post": {
				id:            "createProfile",
				statuses:      []string{"201", "400", "401", "406", "409", "413", "415", "422", "500", "503"},
				secured:       true,
				requestBody:   true,
				successStatus: "201",
				header:        "Location",
			},
			"get": {
				id:            "getProfile",
				statuses:      []string{"200", "401", "404", "406", "500", "503"},
				secured:       true,
				successStatus: "200",
			},
			"patch": {
				id:            "updateProfile",
				statuses:      []string{"200", "400", "401", "404", "406", "413", "415", "422", "500", "503"},
				secured:       true,
				requestBody:   true,
				successStatus: "200",
			},
			"delete": {
				id:            "deleteProfile",
				statuses:      []string{"204", "401", "404", "406", "500", "503"},
				secured:       true,
				successStatus: "204",
			},
		},
	}
	assertExactOperations(t, paths, expected)
	assertProblemMediaTypes(t, paths)
	assertSchemas(t, components)
}

type operationContract struct {
	id            string
	statuses      []string
	secured       bool
	requestBody   bool
	successStatus string
	header        string
}

func assertExactOperations(t *testing.T, paths map[string]any, expected map[string]map[string]operationContract) {
	t.Helper()
	if !reflect.DeepEqual(sortedKeys(paths), sortedKeys(expected)) {
		t.Fatalf("paths = %v, want %v", sortedKeys(paths), sortedKeys(expected))
	}
	operationIDs := make(map[string]string)
	for pathName, methods := range expected {
		path := requireObject(t, paths[pathName])
		if !reflect.DeepEqual(sortedKeys(path), sortedKeys(methods)) {
			t.Fatalf("%s methods = %v, want %v", pathName, sortedKeys(path), sortedKeys(methods))
		}
		for method, contract := range methods {
			operation := requireObject(t, path[method])
			if operation["operationId"] != contract.id {
				t.Fatalf("%s %s operationId = %v, want %q", method, pathName, operation["operationId"], contract.id)
			}
			if previous, exists := operationIDs[contract.id]; exists {
				t.Fatalf("duplicate operationId %q on %s and %s %s", contract.id, previous, method, pathName)
			}
			operationIDs[contract.id] = method + " " + pathName

			responses := requireObject(t, operation["responses"])
			if !reflect.DeepEqual(sortedKeys(responses), contract.statuses) {
				t.Fatalf("%s %s statuses = %v, want %v", method, pathName, sortedKeys(responses), contract.statuses)
			}
			assertSecurity(t, method, pathName, operation, contract.secured)
			assertRequestBody(t, method, pathName, operation, contract.requestBody)
			assertSuccessResponse(t, method, pathName, responses, contract)
			if contract.secured {
				unauthorized := requireObject(t, responses["401"])
				headers := requireObject(t, unauthorized["headers"])
				if _, exists := headers["WWW-Authenticate"]; !exists {
					t.Fatalf("%s %s 401 lacks WWW-Authenticate header", method, pathName)
				}
			}
		}
	}
}

func assertSchemas(t *testing.T, components map[string]any) {
	t.Helper()
	schemas := requireObject(t, components["schemas"])
	for _, name := range []string{"hello.CreateInput", "profile.CreateInput", "profile.UpdateInput"} {
		if requireObject(t, schemas[name])["additionalProperties"] != false {
			t.Fatalf("%s must reject additional properties", name)
		}
	}
	if requireObject(t, schemas["profile.UpdateInput"])["minProperties"] != float64(1) {
		t.Fatal("profile.UpdateInput must require at least one property")
	}

	required := map[string][]string{
		"hello.Data": {"message"},
		"internal_http_v1_profile.Profile": {
			"id",
			"firstname",
			"lastname",
			"email",
			"phoneNumber",
			"marketing",
			"createdAt",
			"updatedAt",
		},
		"items.Item":     {"id", "name", "category", "price", "inStock", "createdAt", "description"},
		"items.ListData": {"items", "total"},
		"items.Money":    {"amountMinor", "currency"},
	}
	for name, fields := range required {
		schema := requireObject(t, schemas[name])
		if schema["additionalProperties"] != false {
			t.Fatalf("%s must be closed", name)
		}
		got, ok := schema["required"].([]any)
		if !ok {
			t.Fatalf("%s required = %#v", name, schema["required"])
		}
		gotFields := make([]string, len(got))
		for i, value := range got {
			gotFields[i], _ = value.(string)
		}
		slices.Sort(gotFields)
		want := slices.Clone(fields)
		slices.Sort(want)
		if !reflect.DeepEqual(gotFields, want) {
			t.Fatalf("%s required = %v, want %v", name, gotFields, want)
		}
	}

	profileProperties := requireObject(t, requireObject(t, schemas["internal_http_v1_profile.Profile"])["properties"])
	if requireObject(t, profileProperties["email"])["format"] != "email" {
		t.Fatal("profile email must use the email format")
	}
	for _, property := range []string{"createdAt", "updatedAt"} {
		if requireObject(t, profileProperties[property])["format"] != "date-time" {
			t.Fatalf("profile %s must use the date-time format", property)
		}
	}
}

func assertSecurity(t *testing.T, method, path string, operation map[string]any, secured bool) {
	t.Helper()
	security, exists := operation["security"]
	if !secured {
		if exists {
			t.Fatalf("%s %s unexpectedly has security: %#v", method, path, security)
		}
		return
	}
	requirements, ok := security.([]any)
	if !ok || len(requirements) != 1 {
		t.Fatalf("%s %s security = %#v", method, path, security)
	}
	requirement := requireObject(t, requirements[0])
	if !reflect.DeepEqual(sortedKeys(requirement), []string{"BearerAuth"}) {
		t.Fatalf("%s %s security = %#v", method, path, security)
	}
}

func assertRequestBody(t *testing.T, method, path string, operation map[string]any, expected bool) {
	t.Helper()
	requestBody, exists := operation["requestBody"]
	if !expected {
		if exists {
			t.Fatalf("%s %s unexpectedly has request body", method, path)
		}
		return
	}
	content := requireObject(t, requireObject(t, requestBody)["content"])
	if !reflect.DeepEqual(sortedKeys(content), []string{"application/json"}) {
		t.Fatalf("%s %s request media = %v", method, path, sortedKeys(content))
	}
}

func assertSuccessResponse(
	t *testing.T,
	method, path string,
	responses map[string]any,
	contract operationContract,
) {
	t.Helper()
	response := requireObject(t, responses[contract.successStatus])
	if contract.successStatus == "204" {
		if _, exists := response["content"]; exists {
			t.Fatalf("%s %s 204 unexpectedly has content", method, path)
		}
	} else {
		content := requireObject(t, response["content"])
		want := []string{"application/cbor", "application/json"}
		if !reflect.DeepEqual(sortedKeys(content), want) {
			t.Fatalf("%s %s success media = %v, want %v", method, path, sortedKeys(content), want)
		}
	}
	if contract.header != "" {
		headers := requireObject(t, response["headers"])
		if _, exists := headers[contract.header]; !exists {
			t.Fatalf("%s %s lacks %s header", method, path, contract.header)
		}
	}
}

func sortedKeys[T any](values map[string]T) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
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
