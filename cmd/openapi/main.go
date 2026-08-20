package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
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
	_, generatedBody := generated["requestBody"]
	_, expectedBody := expected["requestBody"]
	if generatedBody != expectedBody {
		return fmt.Errorf("%s request-body presence is %v, want %v", location, generatedBody, expectedBody)
	}
	generatedParameters, ok := generated["parameters"].([]any)
	if !ok {
		return fmt.Errorf("%s parameters is not an array", location)
	}
	expectedParameters, ok := expected["parameters"].([]any)
	if !ok || !sameParameterIdentities(generatedParameters, expectedParameters) {
		return fmt.Errorf("%s native parameters do not match the normalized contract", location)
	}
	protected := strings.Contains(location, "/v1/profile")
	security, hasSecurity := generated["security"].([]any)
	if protected {
		if !hasSecurity || len(security) != 1 {
			return fmt.Errorf("%s does not require exactly one native security scheme", location)
		}
		requirement, ok := security[0].(map[string]any)
		if !ok || requirement["BearerAuth"] == nil {
			return fmt.Errorf("%s does not require BearerAuth", location)
		}
	} else if hasSecurity && len(security) != 0 {
		return fmt.Errorf("%s is not natively public", location)
	}
	return nil
}

func sameParameterIdentities(generated, expected []any) bool {
	want := make(map[string]struct{}, len(expected))
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
		location, locationOK := parameter["in"].(string)
		parameterName, nameOK := parameter["name"].(string)
		if !locationOK || !nameOK {
			return false
		}
		want[location+":"+parameterName] = struct{}{}
	}
	got := make(map[string]struct{}, len(generated))
	for _, value := range generated {
		parameter, ok := value.(map[string]any)
		if !ok {
			return false
		}
		name, nameOK := parameter["name"].(string)
		location, locationOK := parameter["in"].(string)
		if !nameOK || !locationOK {
			return false
		}
		got[location+":"+name] = struct{}{}
	}
	return sameKeys(got, want)
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
