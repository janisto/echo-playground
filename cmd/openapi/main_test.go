package main

import "testing"

func TestPatchSecurityScheme(t *testing.T) {
	document := map[string]any{}
	patchSecurityScheme(document)

	components := object(document, "components")
	schemes := object(components, "securitySchemes")
	bearer, ok := schemes["BearerAuth"].(map[string]any)
	if !ok {
		t.Fatalf("unexpected bearer scheme: %#v", schemes["BearerAuth"])
	}
	if bearer["type"] != "http" || bearer["scheme"] != "bearer" || bearer["bearerFormat"] != "JWT" {
		t.Fatalf("unexpected bearer scheme: %#v", bearer)
	}
}

func TestPatchProblemMediaTypes(t *testing.T) {
	document := map[string]any{
		"paths": map[string]any{
			"/example": map[string]any{
				"get": map[string]any{
					"responses": map[string]any{
						"200":     responseContent("application/json", "application/cbor"),
						"400":     responseContent("application/json", "application/cbor"),
						"default": responseContent("application/json"),
					},
				},
			},
		},
	}

	patchProblemMediaTypes(document)

	responses := object(
		object(object(object(document, "paths"), "/example"), "get"),
		"responses",
	)
	success := object(object(responses, "200"), "content")
	if _, ok := success["application/json"]; !ok {
		t.Fatalf("success media type changed: %#v", success)
	}
	failure := object(object(responses, "400"), "content")
	for _, mediaType := range []string{"application/problem+json", "application/problem+cbor"} {
		if _, ok := failure[mediaType]; !ok {
			t.Fatalf("failure lacks %s: %#v", mediaType, failure)
		}
	}
	for _, mediaType := range []string{"application/json", "application/cbor"} {
		if _, ok := failure[mediaType]; ok {
			t.Fatalf("failure retained %s: %#v", mediaType, failure)
		}
	}
	defaultContent := object(object(responses, "default"), "content")
	if _, ok := defaultContent["application/json"]; !ok {
		t.Fatalf("default response unexpectedly changed: %#v", defaultContent)
	}
}

func responseContent(mediaTypes ...string) map[string]any {
	content := make(map[string]any, len(mediaTypes))
	for _, mediaType := range mediaTypes {
		content[mediaType] = map[string]any{"schema": map[string]any{"type": "object"}}
	}
	return map[string]any{"content": content}
}
