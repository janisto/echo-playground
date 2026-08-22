package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"maps"
	"os"
	"sort"
	"strings"

	"sigs.k8s.io/yaml"
)

const (
	defaultInputPath = "api-docs/swagger.json"
	defaultJSONPath  = "api-docs/swagger.json"
	defaultYAMLPath  = "api-docs/swagger.yaml"
)

var nativeRequestBodyReferences = map[string]string{
	"createHello":   "#/components/schemas/hello.CreateInput",
	"createProfile": "#/components/schemas/profile.CreateInput",
	"updateProfile": "#/components/schemas/profile.UpdateInput",
}

func main() {
	inputPath := flag.String("input", defaultInputPath, "Swag-generated OpenAPI JSON input")
	jsonPath := flag.String("json", defaultJSONPath, "normalized OpenAPI JSON output")
	yamlPath := flag.String("yaml", defaultYAMLPath, "normalized OpenAPI YAML output")
	flag.Parse()
	if err := run(*inputPath, *jsonPath, *yamlPath); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "openapi: %v\n", err)
		os.Exit(1)
	}
}

func run(inputPath, jsonPath, yamlPath string) error {
	// #nosec G304 -- paths are explicit repository-tool arguments, not application request data.
	input, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("read Swag document: %w", err)
	}
	var generated map[string]any
	decodeErr := json.Unmarshal(input, &generated)
	if decodeErr != nil {
		return fmt.Errorf("decode Swag document: %w", decodeErr)
	}
	document, err := normalizeDocument(generated)
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(document, "", "    ")
	if err != nil {
		return fmt.Errorf("encode OpenAPI JSON: %w", err)
	}
	data = append(data, '\n')
	//nolint:gosec // Generated public artifact must be readable in the repository.
	writeJSONErr := os.WriteFile(jsonPath, data, 0o644)
	if writeJSONErr != nil {
		return fmt.Errorf("write OpenAPI JSON: %w", writeJSONErr)
	}
	yamlData, err := yaml.JSONToYAML(data)
	if err != nil {
		return fmt.Errorf("encode OpenAPI YAML: %w", err)
	}
	//nolint:gosec // Generated public artifact must be readable in the repository.
	writeYAMLErr := os.WriteFile(yamlPath, yamlData, 0o644)
	if writeYAMLErr != nil {
		return fmt.Errorf("write OpenAPI YAML: %w", writeYAMLErr)
	}
	return nil
}

func normalizeDocument(generated map[string]any) (map[string]any, error) {
	paths := portablePaths()
	if err := validateNativeDocument(generated, paths); err != nil {
		return nil, fmt.Errorf("validate Swag registration: %w", err)
	}
	generatedInfo, ok := generated["info"].(map[string]any)
	if !ok {
		return nil, errors.New("generated info is not an object")
	}
	title, titleOK := generatedInfo["title"].(string)
	version, versionOK := generatedInfo["version"].(string)
	if !titleOK || strings.TrimSpace(title) == "" || !versionOK || strings.TrimSpace(version) == "" {
		return nil, errors.New("generated info title and version must be non-empty")
	}
	info := map[string]any{"title": title, "version": version}
	if description, ok := generatedInfo["description"].(string); ok && description != "" {
		info["description"] = description
	}
	return map[string]any{
		"openapi":           "3.1.2",
		"jsonSchemaDialect": "https://json-schema.org/draft/2020-12/schema",
		"info":              info,
		"paths":             paths,
		"components": map[string]any{
			"schemas":         portableSchemas(),
			"parameters":      parameterComponents(),
			"headers":         headerComponents(),
			"securitySchemes": securitySchemes(),
		},
	}, nil
}

func validateNativeDocument(generated, expectedPaths map[string]any) error {
	if securityValue, present := generated["security"]; present {
		security, ok := securityValue.([]any)
		if !ok || len(security) != 0 {
			return errors.New("document-level security must be absent or empty")
		}
	}
	generatedPaths, ok := generated["paths"].(map[string]any)
	if !ok {
		return errors.New("paths is not an object")
	}
	if len(generatedPaths) != len(expectedPaths) {
		return fmt.Errorf("path count is %d, want %d", len(generatedPaths), len(expectedPaths))
	}
	for path, expectedPathValue := range expectedPaths {
		generatedPath, ok := generatedPaths[path].(map[string]any)
		if !ok {
			return fmt.Errorf("missing path %s", path)
		}
		expectedPath, ok := expectedPathValue.(map[string]any)
		if !ok {
			return fmt.Errorf("normalized path %s is not an object", path)
		}
		if len(generatedPath) != len(expectedPath) {
			return fmt.Errorf("%s method count is %d, want %d", path, len(generatedPath), len(expectedPath))
		}
		for method, expectedOperationValue := range expectedPath {
			generatedOperation, ok := generatedPath[method].(map[string]any)
			if !ok {
				return fmt.Errorf("missing operation %s %s", method, path)
			}
			expectedOperation, ok := expectedOperationValue.(map[string]any)
			if !ok {
				return fmt.Errorf("normalized operation %s %s is not an object", method, path)
			}
			if generatedOperation["operationId"] != expectedOperation["operationId"] {
				return fmt.Errorf(
					"%s %s operationId is %v, want %v",
					method,
					path,
					generatedOperation["operationId"],
					expectedOperation["operationId"],
				)
			}
			if err := validateNativeOperation(method+" "+path, generatedOperation, expectedOperation); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateNativeOperation(location string, generated, expected map[string]any) error {
	generatedResponses, ok := generated["responses"].(map[string]any)
	if !ok {
		return fmt.Errorf("%s responses is not an object", location)
	}
	expectedResponses, ok := expected["responses"].(map[string]any)
	if !ok {
		return fmt.Errorf("%s normalized responses is not an object", location)
	}
	if !sameKeys(generatedResponses, expectedResponses) {
		return fmt.Errorf(
			"%s response statuses are %v, want %v",
			location,
			mapKeys(generatedResponses),
			mapKeys(expectedResponses),
		)
	}
	if err := validateNativeRequestBody(location, generated, expected); err != nil {
		return err
	}
	generatedParameters, ok := generated["parameters"].([]any)
	if !ok {
		return fmt.Errorf("%s parameters is not an array", location)
	}
	expectedParameters, ok := expected["parameters"].([]any)
	if !ok || !sameParameterDefinitions(generatedParameters, expectedParameters) {
		return fmt.Errorf("%s native parameters do not match the normalized contract", location)
	}
	if err := validateNativeSecurity(location, generated, expected); err != nil {
		return err
	}
	return nil
}

func validateNativeRequestBody(location string, generated, expected map[string]any) error {
	generatedBodyValue, generatedBodyPresent := generated["requestBody"]
	_, expectedBodyPresent := expected["requestBody"]
	if generatedBodyPresent != expectedBodyPresent {
		return fmt.Errorf(
			"%s request-body presence is %v, want %v",
			location,
			generatedBodyPresent,
			expectedBodyPresent,
		)
	}
	if !expectedBodyPresent {
		return nil
	}
	operationID, ok := expected["operationId"].(string)
	if !ok {
		return fmt.Errorf("%s normalized operationId is not a string", location)
	}
	expectedReference, ok := nativeRequestBodyReferences[operationID]
	if !ok {
		return fmt.Errorf("%s has no native request-body schema registration", location)
	}
	generatedBody, ok := generatedBodyValue.(map[string]any)
	if !ok {
		return fmt.Errorf("%s native request body is not an object", location)
	}
	required, ok := generatedBody["required"].(bool)
	if !ok || !required {
		return fmt.Errorf("%s native request body is not required", location)
	}
	content, ok := generatedBody["content"].(map[string]any)
	if !ok || !sameKeys(content, map[string]struct{}{
		"application/json": {},
		"application/cbor": {},
	}) {
		return fmt.Errorf(
			"%s native request media are %v, want application/json and application/cbor",
			location,
			mapKeys(content),
		)
	}
	jsonMedia, ok := content["application/json"].(map[string]any)
	if !ok {
		return fmt.Errorf("%s native JSON request media is not an object", location)
	}
	jsonSchema, ok := jsonMedia["schema"].(map[string]any)
	if !ok || len(jsonSchema) != 1 {
		return fmt.Errorf("%s native JSON request schema is not exactly oneOf", location)
	}
	variants, ok := jsonSchema["oneOf"].([]any)
	if !ok || len(variants) != 2 {
		return fmt.Errorf("%s native JSON request schema does not have the expected two registrations", location)
	}
	baseVariant, ok := variants[0].(map[string]any)
	if !ok || len(baseVariant) != 1 || baseVariant["type"] != "object" {
		return fmt.Errorf("%s native JSON request schema lacks the generator object registration", location)
	}
	dtoVariant, ok := variants[1].(map[string]any)
	summary, summaryOK := dtoVariant["summary"].(string)
	description, descriptionOK := dtoVariant["description"].(string)
	if !ok || !sameKeys(dtoVariant, map[string]struct{}{
		"$ref": {}, "summary": {}, "description": {},
	}) || dtoVariant["$ref"] != expectedReference || !summaryOK || summary == "" ||
		!descriptionOK || description == "" {
		return fmt.Errorf("%s native JSON request schema does not exactly reference %s", location, expectedReference)
	}
	cborMedia, ok := content["application/cbor"].(map[string]any)
	if !ok {
		return fmt.Errorf("%s native CBOR request media is not an object", location)
	}
	cborSchema, ok := cborMedia["schema"].(map[string]any)
	if !ok || len(cborSchema) != 1 || cborSchema["type"] != "string" {
		return fmt.Errorf("%s native CBOR request schema does not match the generator registration", location)
	}
	return nil
}

func validateNativeSecurity(location string, generated, expected map[string]any) error {
	expectedSecurity, ok := expected["security"].([]any)
	if !ok {
		return fmt.Errorf("%s normalized security is not an array", location)
	}
	generatedSecurityValue, generatedSecurityPresent := generated["security"]
	if len(expectedSecurity) == 0 {
		if !generatedSecurityPresent {
			return nil
		}
		generatedSecurity, securityOK := generatedSecurityValue.([]any)
		if !securityOK || len(generatedSecurity) != 0 {
			return fmt.Errorf("%s is not natively public", location)
		}
		return nil
	}
	generatedSecurity, securityOK := generatedSecurityValue.([]any)
	if !generatedSecurityPresent || !securityOK || len(generatedSecurity) != 1 {
		return fmt.Errorf("%s does not require exactly one native security requirement", location)
	}
	requirement, ok := generatedSecurity[0].(map[string]any)
	if !ok || len(requirement) != 1 {
		return fmt.Errorf("%s native security requirement is not exactly BearerAuth", location)
	}
	scopes, ok := requirement["BearerAuth"].([]any)
	if !ok || len(scopes) != 0 {
		return fmt.Errorf("%s native BearerAuth scopes are not empty", location)
	}
	return nil
}

func sameParameterDefinitions(generated, expected []any) bool {
	if len(generated) != len(expected) {
		return false
	}
	want := make([]map[string]any, 0, len(expected))
	components := parameterComponents()
	for _, value := range expected {
		referenceObject, ok := value.(map[string]any)
		if !ok {
			return false
		}
		reference, ok := referenceObject["$ref"].(string)
		if !ok {
			return false
		}
		name := strings.TrimPrefix(reference, "#/components/parameters/")
		parameter, ok := components[name].(map[string]any)
		if !ok {
			return false
		}
		want = append(want, parameter)
	}
	matched := make([]bool, len(want))
	for _, value := range generated {
		parameter, ok := value.(map[string]any)
		if !ok {
			return false
		}
		found := false
		for index, expectedParameter := range want {
			if !matched[index] && sameNativeParameterDefinition(parameter, expectedParameter) {
				matched[index] = true
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func sameNativeParameterDefinition(generated, expected map[string]any) bool {
	got, ok := nativeParameterSemantics(generated)
	if !ok {
		return false
	}
	want, ok := nativeParameterSemantics(expected)
	if !ok {
		return false
	}
	gotSchema, gotSchemaOK := got["schema"].(map[string]any)
	wantSchema, wantSchemaOK := want["schema"].(map[string]any)
	if !gotSchemaOK || !wantSchemaOK {
		return false
	}
	// Swag does not emit pattern for primitive @Param annotations. If a future
	// generator emits it, the native value must match the normalized contract.
	if _, present := gotSchema["pattern"]; !present {
		delete(wantSchema, "pattern")
	}
	gotData, gotErr := json.Marshal(got)
	wantData, wantErr := json.Marshal(want)
	return gotErr == nil && wantErr == nil && string(gotData) == string(wantData)
}

func nativeParameterSemantics(parameter map[string]any) (map[string]any, bool) {
	for key := range parameter {
		switch key {
		case "description", "in", "name", "required", "schema":
		default:
			return nil, false
		}
	}
	name, nameOK := parameter["name"].(string)
	location, locationOK := parameter["in"].(string)
	schema, schemaOK := parameter["schema"].(map[string]any)
	if !nameOK || !locationOK || !schemaOK {
		return nil, false
	}
	required := false
	if value, present := parameter["required"]; present {
		var requiredOK bool
		required, requiredOK = value.(bool)
		if !requiredOK {
			return nil, false
		}
	}
	return map[string]any{
		"in":       location,
		"name":     name,
		"required": required,
		"schema":   maps.Clone(schema),
	}, true
}

func sameKeys[Left, Right any](left map[string]Left, right map[string]Right) bool {
	if len(left) != len(right) {
		return false
	}
	for key := range left {
		if _, ok := right[key]; !ok {
			return false
		}
	}
	return true
}

func mapKeys(values map[string]any) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func ref(section, name string) map[string]any {
	return map[string]any{"$ref": "#/components/" + section + "/" + name}
}
