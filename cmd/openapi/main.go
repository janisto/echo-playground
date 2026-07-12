package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"sigs.k8s.io/yaml"
)

const (
	jsonPath = "api-docs/swagger.json"
	yamlPath = "api-docs/swagger.yaml"
)

func main() {
	if err := run(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "openapi: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		return fmt.Errorf("read generated JSON: %w", err)
	}

	var document map[string]any
	if decodeErr := json.Unmarshal(data, &document); decodeErr != nil {
		return fmt.Errorf("decode generated JSON: %w", decodeErr)
	}
	patchSecurityScheme(document)
	patchProblemMediaTypes(document)

	data, err = json.MarshalIndent(document, "", "    ")
	if err != nil {
		return fmt.Errorf("encode JSON: %w", err)
	}
	data = append(data, '\n')
	//nolint:gosec // Generated public artifact must be readable in the repository.
	if writeErr := os.WriteFile(jsonPath, data, 0o644); writeErr != nil {
		return fmt.Errorf("write JSON: %w", writeErr)
	}

	yamlData, err := yaml.JSONToYAML(data)
	if err != nil {
		return fmt.Errorf("encode YAML: %w", err)
	}
	//nolint:gosec // Generated public artifact must be readable in the repository.
	if writeErr := os.WriteFile(
		yamlPath,
		yamlData,
		0o644,
	); writeErr != nil {
		return fmt.Errorf("write YAML: %w", writeErr)
	}
	return nil
}

func patchSecurityScheme(document map[string]any) {
	components := object(document, "components")
	schemes := object(components, "securitySchemes")
	schemes["BearerAuth"] = map[string]any{
		"type":         "http",
		"scheme":       "bearer",
		"bearerFormat": "JWT",
	}
}

func patchProblemMediaTypes(document map[string]any) {
	for _, pathValue := range object(document, "paths") {
		path, ok := pathValue.(map[string]any)
		if !ok {
			continue
		}
		for _, operationValue := range path {
			operation, ok := operationValue.(map[string]any)
			if !ok {
				continue
			}
			for status, responseValue := range object(operation, "responses") {
				statusCode, err := strconv.Atoi(status)
				if err != nil || statusCode < 400 {
					continue
				}
				response, ok := responseValue.(map[string]any)
				if !ok {
					continue
				}
				content := object(response, "content")
				rename(content, "application/json", "application/problem+json")
				rename(content, "application/cbor", "application/problem+cbor")
			}
		}
	}
}

func object(parent map[string]any, key string) map[string]any {
	if child, ok := parent[key].(map[string]any); ok {
		return child
	}
	child := make(map[string]any)
	parent[key] = child
	return child
}

func rename(values map[string]any, oldKey, newKey string) {
	value, ok := values[oldKey]
	if !ok {
		return
	}
	delete(values, oldKey)
	values[newKey] = value
}
