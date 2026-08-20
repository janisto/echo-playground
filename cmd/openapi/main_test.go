package main

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
)

var portableInventory = map[string]string{
	"get /health":                                   "getHealth",
	"get /v1/hello":                                 "getHello",
	"post /v1/hello":                                "createHello",
	"get /v1/items":                                 "listItems",
	"post /v1/profile":                              "createProfile",
	"get /v1/profile":                               "getProfile",
	"patch /v1/profile":                             "updateProfile",
	"delete /v1/profile":                            "deleteProfile",
	"get /v1/github/owners/{owner}":                 "getGitHubOwner",
	"get /v1/github/owners/{owner}/repos":           "listGitHubOwnerRepositories",
	"get /v1/github/repos/{owner}/{repo}":           "getGitHubRepository",
	"get /v1/github/repos/{owner}/{repo}/activity":  "listGitHubRepositoryActivity",
	"get /v1/github/repos/{owner}/{repo}/languages": "listGitHubRepositoryLanguages",
	"get /v1/github/repos/{owner}/{repo}/tags":      "listGitHubRepositoryTags",
}

var nativeRequestSchemaByOperation = map[string]string{
	"createHello":   "#/components/schemas/hello.CreateInput",
	"createProfile": "#/components/schemas/profile.CreateInput",
	"updateProfile": "#/components/schemas/profile.UpdateInput",
}

func TestGeneratedDocumentMatchesNormalizer(t *testing.T) {
	generated := generatedDocument(t)
	want, err := normalizeDocument(nativeRegistrationFixture())
	if err != nil {
		t.Fatalf("normalize native registration fixture: %v", err)
	}
	wantData, err := json.Marshal(want)
	if err != nil {
		t.Fatalf("encode normalized document: %v", err)
	}
	var normalized map[string]any
	if err := json.Unmarshal(wantData, &normalized); err != nil {
		t.Fatalf("normalize built document: %v", err)
	}
	if !reflect.DeepEqual(generated, normalized) {
		t.Fatal("committed OpenAPI document is stale; run just docs")
	}
}

func TestNormalizerRejectsNativeRegistrationDrift(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(*testing.T, map[string]any)
	}{
		{name: "operation ID", mutate: func(t *testing.T, document map[string]any) {
			t.Helper()
			operation := nativeOperation(t, document, "/health", "get")
			operation["operationId"] = "driftedHealth"
		}},
		{name: "response status", mutate: func(t *testing.T, document map[string]any) {
			t.Helper()
			delete(mapValue(t, nativeOperation(t, document, "/v1/hello", "post"), "responses"), "413")
		}},
		{name: "missing protected security", mutate: func(t *testing.T, document map[string]any) {
			t.Helper()
			delete(nativeOperation(t, document, "/v1/profile", "get"), "security")
		}},
		{name: "additional protected security scheme", mutate: func(t *testing.T, document map[string]any) {
			t.Helper()
			security := arrayValue(t, nativeOperation(t, document, "/v1/profile", "get")["security"], "security")
			objectValue(t, security[0], "security requirement")["ApiKey"] = []any{}
		}},
		{name: "nonempty bearer scope", mutate: func(t *testing.T, document map[string]any) {
			t.Helper()
			security := arrayValue(t, nativeOperation(t, document, "/v1/profile", "get")["security"], "security")
			objectValue(t, security[0], "security requirement")["BearerAuth"] = []any{"profile:read"}
		}},
		{name: "public security requirement", mutate: func(t *testing.T, document map[string]any) {
			t.Helper()
			nativeOperation(t, document, "/health", "get")["security"] = []any{map[string]any{"BearerAuth": []any{}}}
		}},
		{name: "inherited public security requirement", mutate: func(t *testing.T, document map[string]any) {
			t.Helper()
			document["security"] = []any{map[string]any{"BearerAuth": []any{}}}
		}},
		{name: "malformed document security", mutate: func(t *testing.T, document map[string]any) {
			t.Helper()
			document["security"] = map[string]any{"BearerAuth": []any{}}
		}},
		{name: "optional request body", mutate: func(t *testing.T, document map[string]any) {
			t.Helper()
			mapValue(t, nativeOperation(t, document, "/v1/hello", "post"), "requestBody")["required"] = false
		}},
		{name: "missing request media", mutate: func(t *testing.T, document map[string]any) {
			t.Helper()
			body := mapValue(t, nativeOperation(t, document, "/v1/hello", "post"), "requestBody")
			delete(mapValue(t, body, "content"), "application/cbor")
		}},
		{name: "additional request media", mutate: func(t *testing.T, document map[string]any) {
			t.Helper()
			body := mapValue(t, nativeOperation(t, document, "/v1/hello", "post"), "requestBody")
			mapValue(t, body, "content")["text/plain"] = map[string]any{"schema": map[string]any{"type": "string"}}
		}},
		{name: "wrong request DTO", mutate: func(t *testing.T, document map[string]any) {
			t.Helper()
			body := mapValue(t, nativeOperation(t, document, "/v1/profile", "post"), "requestBody")
			jsonMedia := mapValue(t, mapValue(t, body, "content"), "application/json")
			variants := arrayValue(t, mapValue(t, jsonMedia, "schema")["oneOf"], "request schema variants")
			objectValue(t, variants[1], "request DTO")["$ref"] = "#/components/schemas/profile.UpdateInput"
		}},
		{name: "body on bodyless operation", mutate: func(t *testing.T, document map[string]any) {
			t.Helper()
			nativeOperation(t, document, "/health", "get")["requestBody"] = nativeRequestBodyFixture(
				"#/components/schemas/hello.CreateInput",
			)
		}},
		{name: "parameter requiredness", mutate: func(t *testing.T, document map[string]any) {
			t.Helper()
			parameters := arrayValue(t, nativeOperation(t, document, "/v1/github/owners/{owner}", "get")["parameters"], "parameters")
			objectValue(t, parameters[1], "owner parameter")["required"] = false
		}},
		{name: "parameter schema type", mutate: func(t *testing.T, document map[string]any) {
			t.Helper()
			parameters := arrayValue(t, nativeOperation(t, document, "/v1/items", "get")["parameters"], "parameters")
			mapValue(t, objectValue(t, parameters[1], "limit parameter"), "schema")["type"] = "number"
		}},
		{name: "parameter schema bound", mutate: func(t *testing.T, document map[string]any) {
			t.Helper()
			parameters := arrayValue(t, nativeOperation(t, document, "/v1/items", "get")["parameters"], "parameters")
			mapValue(t, objectValue(t, parameters[2], "cursor parameter"), "schema")["maxLength"] = 2049
		}},
		{name: "parameter schema default", mutate: func(t *testing.T, document map[string]any) {
			t.Helper()
			parameters := arrayValue(t, nativeOperation(t, document, "/v1/items", "get")["parameters"], "parameters")
			mapValue(t, objectValue(t, parameters[1], "limit parameter"), "schema")["default"] = 21
		}},
		{name: "parameter schema enum", mutate: func(t *testing.T, document map[string]any) {
			t.Helper()
			parameters := arrayValue(t, nativeOperation(t, document, "/v1/items", "get")["parameters"], "parameters")
			mapValue(t, objectValue(t, parameters[3], "category parameter"), "schema")["enum"] = []any{"electronics"}
		}},
		{name: "parameter schema pattern", mutate: func(t *testing.T, document map[string]any) {
			t.Helper()
			parameters := arrayValue(t, nativeOperation(t, document, "/v1/items", "get")["parameters"], "parameters")
			mapValue(t, objectValue(t, parameters[2], "cursor parameter"), "schema")["pattern"] = `^wrong$`
		}},
		{name: "duplicate parameter", mutate: func(t *testing.T, document map[string]any) {
			t.Helper()
			operation := nativeOperation(t, document, "/v1/items", "get")
			parameters := arrayValue(t, operation["parameters"], "parameters")
			operation["parameters"] = append(parameters, parameters[0])
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			document := nativeRegistrationFixture()
			test.mutate(t, document)
			if _, err := normalizeDocument(document); err == nil {
				t.Fatal("normalizeDocument accepted native registration drift")
			}
		})
	}
}

func TestNormalizerAcceptsExplicitEmptyNativeDocumentSecurity(t *testing.T) {
	document := nativeRegistrationFixture()
	document["security"] = []any{}
	if _, err := normalizeDocument(document); err != nil {
		t.Fatalf("normalizeDocument rejected explicit empty document security: %v", err)
	}
}

func TestPortableOperationInventoryAndSecurity(t *testing.T) {
	document := generatedDocument(t)
	paths := mapValue(t, document, "paths")
	seenIDs := make(map[string]struct{})
	count := 0
	for path, pathValue := range paths {
		for method, operationValue := range objectValue(t, pathValue, path) {
			operation := objectValue(t, operationValue, method+" "+path)
			key := method + " " + path
			wantID, portable := portableInventory[key]
			if !portable {
				t.Fatalf("unexpected documented operation %s", key)
			}
			if operation["operationId"] != wantID {
				t.Fatalf("%s operationId = %v, want %s", key, operation["operationId"], wantID)
			}
			if _, duplicate := seenIDs[wantID]; duplicate {
				t.Fatalf("duplicate operationId %s", wantID)
			}
			seenIDs[wantID] = struct{}{}
			security := arrayValue(t, operation["security"], key+" security")
			if path == "/v1/profile" {
				if len(security) != 1 {
					t.Fatalf("%s must require only BearerAuth: %#v", key, security)
				}
				requirement := objectValue(t, security[0], key+" security requirement")
				if len(requirement) != 1 {
					t.Fatalf("%s must have one security scheme: %#v", key, requirement)
				}
				scopes := arrayValue(t, requirement["BearerAuth"], key+" BearerAuth scopes")
				if len(scopes) != 0 {
					t.Fatalf("%s BearerAuth scopes = %#v", key, scopes)
				}
			} else if len(security) != 0 {
				t.Fatalf("public operation %s has security %#v", key, security)
			}
			count++
		}
	}
	if count != len(portableInventory) {
		t.Fatalf("documented operation count = %d, want %d", count, len(portableInventory))
	}
}

func TestOperationsDescribeExactMediaFailuresAndHeaders(t *testing.T) {
	document := generatedDocument(t)
	paths := mapValue(t, document, "paths")
	for key := range portableInventory {
		method, path, _ := strings.Cut(key, " ")
		operation := mapValue(t, mapValue(t, paths, path), method)
		parameters := arrayValue(t, operation["parameters"], key+" parameters")
		if len(parameters) == 0 ||
			objectValue(t, parameters[0], key+" request ID parameter")["$ref"] != "#/components/parameters/XRequestID" {
			t.Fatalf("%s lacks X-Request-ID input", key)
		}
		for status, responseValue := range mapValue(t, operation, "responses") {
			response := objectValue(t, responseValue, key+" response "+status)
			headers := mapValue(t, response, "headers")
			for _, name := range []string{"X-Request-ID", "Cache-Control", "X-Content-Type-Options", "X-Frame-Options", "Referrer-Policy", "Vary"} {
				if headers[name] == nil {
					t.Fatalf("%s response %s lacks %s", key, status, name)
				}
			}
			content, hasContent := response["content"].(map[string]any)
			if status == "204" {
				if hasContent {
					t.Fatalf("%s 204 advertises content", key)
				}
				continue
			}
			if status[0] == '2' {
				assertMedia(t, key+" "+status, content, "application/json", "application/cbor")
			} else {
				assertMedia(t, key+" "+status, content, "application/problem+json", "application/cbor")
				if content["application/problem+cbor"] != nil {
					t.Fatalf("%s advertises nonportable application/problem+cbor", key)
				}
			}
		}
	}
	profilePost := mapValue(t, mapValue(t, paths, "/v1/profile"), "post")
	assertStatusSet(t, profilePost, "201", "400", "401", "406", "409", "413", "415", "422", "500", "503")
	githubGet := mapValue(t, mapValue(t, paths, "/v1/github/owners/{owner}"), "get")
	assertStatusSet(t, githubGet, "200", "400", "404", "406", "422", "429", "500", "502", "504")
	if githubGet["requestBody"] != nil {
		t.Fatal("GitHub GET advertises a request body")
	}
}

func TestPortableParameterSchemas(t *testing.T) {
	document := generatedDocument(t)
	parameters := mapValue(t, mapValue(t, document, "components"), "parameters")
	want := map[string]map[string]any{
		"XRequestID": {
			"name": "X-Request-ID", "in": "header", "required": false,
			"schema": map[string]any{
				"type": "string", "minLength": float64(1), "maxLength": float64(128),
				"pattern": `^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$`,
			},
		},
		"Owner": {
			"name": "owner", "in": "path", "required": true,
			"schema": map[string]any{
				"type": "string", "minLength": float64(1), "maxLength": float64(39),
				"pattern": `^[A-Za-z0-9](?:[A-Za-z0-9_-]{0,37}[A-Za-z0-9])?$`,
			},
		},
		"Repository": {
			"name": "repo", "in": "path", "required": true,
			"schema": map[string]any{
				"type": "string", "minLength": float64(1), "maxLength": float64(100),
				"pattern": `^(?=.*[A-Za-z0-9_-])[A-Za-z0-9._-]+$`,
			},
		},
		"Limit": {
			"name": "limit", "in": "query", "required": false,
			"schema": map[string]any{
				"type": "integer", "minimum": float64(1), "maximum": float64(100), "default": float64(20),
			},
		},
		"Cursor": {
			"name": "cursor", "in": "query", "required": false,
			"schema": map[string]any{
				"type": "string", "minLength": float64(1), "maxLength": float64(2048), "pattern": `^[!-~]+$`,
			},
		},
		"Category": {
			"name": "category", "in": "query", "required": false,
			"schema": map[string]any{
				"type": "string",
				"enum": []any{"electronics", "tools", "accessories", "robotics", "power", "components"},
			},
		},
	}
	if len(parameters) != len(want) {
		t.Fatalf("parameter component count = %d, want %d", len(parameters), len(want))
	}
	for name, expectation := range want {
		parameter := mapValue(t, parameters, name)
		if parameter["name"] != expectation["name"] || parameter["in"] != expectation["in"] ||
			parameter["required"] != expectation["required"] ||
			!reflect.DeepEqual(mapValue(t, parameter, "schema"), expectation["schema"]) {
			t.Fatalf("parameter %s = %#v, want semantic definition %#v", name, parameter, expectation)
		}
	}
}

func TestPortableSchemasAndReferences(t *testing.T) {
	document := generatedDocument(t)
	if document["openapi"] != "3.1.2" {
		t.Fatalf("openapi = %v", document["openapi"])
	}
	schemas := mapValue(t, mapValue(t, document, "components"), "schemas")
	for _, name := range []string{"Health", "HelloCreate", "Greeting", "Money", "Item", "ItemPage", "ProfileCreate", "ProfileUpdate", "Profile", "GitHubOwner", "GitHubRepository", "GitHubActivityPage", "GitHubLanguages", "GitHubTagPage"} {
		schema := mapValue(t, schemas, name)
		if schema["type"] == "object" && schema["additionalProperties"] != false {
			t.Fatalf("schema %s is not closed", name)
		}
	}
	profileCreate := mapValue(t, schemas, "ProfileCreate")
	if mapValue(t, mapValue(t, profileCreate, "properties"), "termsAccepted")["const"] != true {
		t.Fatal("ProfileCreate does not require literal terms acceptance")
	}
	profileUpdate := mapValue(t, schemas, "ProfileUpdate")
	if profileUpdate["minProperties"] != float64(1) {
		t.Fatal("ProfileUpdate does not require one field")
	}
	for name, schema := range map[string]map[string]any{
		"ProfileCreate": profileCreate,
		"ProfileUpdate": profileUpdate,
	} {
		properties := mapValue(t, schema, "properties")
		if mapValue(t, properties, "contactEmail")["$ref"] != "#/components/schemas/ContactEmailInput" ||
			mapValue(t, properties, "phoneNumber")["$ref"] != "#/components/schemas/PhoneNumberInput" {
			t.Fatalf("%s does not model pre-normalization contact input", name)
		}
	}
	profileProperties := mapValue(t, mapValue(t, schemas, "Profile"), "properties")
	if mapValue(t, profileProperties, "contactEmail")["$ref"] != "#/components/schemas/ContactEmail" ||
		mapValue(t, profileProperties, "phoneNumber")["$ref"] != "#/components/schemas/PhoneNumber" {
		t.Fatal("Profile response does not use canonical contact schemas")
	}
	emailInput := mapValue(t, schemas, "ContactEmailInput")
	emailPattern, ok := emailInput["pattern"].(string)
	if !ok || emailInput["maxLength"] != nil ||
		!strings.HasPrefix(emailPattern, `^[\u0009-\u000D\u0020]*`) ||
		!strings.Contains(emailPattern, `{3,254}`) ||
		!strings.HasSuffix(emailPattern, `[\u0009-\u000D\u0020]*$`) {
		t.Fatalf("ContactEmailInput does not admit exact surrounding ASCII whitespace: %#v", emailInput)
	}
	phoneInput := mapValue(t, schemas, "PhoneNumberInput")
	if phoneInput["pattern"] != `^[\u0009-\u000D\u0020]*\+[1-9][0-9]{6,14}[\u0009-\u000D\u0020]*$` {
		t.Fatalf("PhoneNumberInput pattern = %#v", phoneInput["pattern"])
	}
	itemSchema := mapValue(t, schemas, "Item")
	itemIDs := arrayValue(t, mapValue(t, mapValue(t, itemSchema, "properties"), "id")["enum"], "Item id enum")
	if len(itemIDs) != 30 || itemIDs[0] != "item-001" || itemIDs[29] != "item-030" {
		t.Fatalf("Item id enum = %#v", itemIDs)
	}
	allOf := arrayValue(t, itemSchema["allOf"], "Item allOf")
	if len(allOf) != 1 {
		t.Fatalf("Item allOf length = %d", len(allOf))
	}
	exactItems := arrayValue(t, objectValue(t, allOf[0], "Item allOf member")["oneOf"], "Item exact records")
	if len(exactItems) != 30 {
		t.Fatalf("Item exact record count = %d", len(exactItems))
	}
	firstProperties := mapValue(t, objectValue(t, exactItems[0], "first Item record"), "properties")
	lastProperties := mapValue(t, objectValue(t, exactItems[29], "last Item record"), "properties")
	if mapValue(t, firstProperties, "name")["const"] != "Alpha Widget" ||
		mapValue(t, lastProperties, "description")["const"] != "Gold-plated premium cable" {
		t.Fatal("Item schema does not bind all 30 fixed catalog records")
	}
	ownerProperties := mapValue(t, mapValue(t, schemas, "GitHubOwner"), "properties")
	if mapValue(t, ownerProperties, "name")["minLength"] != float64(1) {
		t.Fatal("GitHubOwner nullable display name permits an empty non-null value")
	}
	repositoryProperties := mapValue(t, mapValue(t, schemas, "GitHubRepository"), "properties")
	license := mapValue(t, repositoryProperties, "license")
	if license["minLength"] != float64(1) || mapValue(t, license, "not")["const"] != "NOASSERTION" {
		t.Fatalf("GitHubRepository license schema = %#v", license)
	}
	activityProperties := mapValue(t, mapValue(t, schemas, "GitHubActivity"), "properties")
	if mapValue(t, activityProperties, "actor")["minLength"] != float64(1) ||
		len(arrayValue(t, mapValue(t, activityProperties, "actorAvatarUrl")["oneOf"], "actorAvatarUrl oneOf")) != 2 {
		t.Fatal("GitHubActivity nullable actor pair is under-constrained")
	}
	walkReferences(t, document, document)
}

func generatedDocument(t *testing.T) map[string]any {
	t.Helper()
	data, err := os.ReadFile("../../api-docs/swagger.json")
	if err != nil {
		t.Fatalf("read generated document: %v", err)
	}
	var document map[string]any
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatalf("decode generated document: %v", err)
	}
	return document
}

func nativeRegistrationFixture() map[string]any {
	paths := make(map[string]any)
	components := parameterComponents()
	for path, pathValue := range portablePaths() {
		operations := make(map[string]any)
		for method, operationValue := range mustObject(pathValue) {
			expected := mustObject(operationValue)
			expectedParameters := mustArray(expected["parameters"])
			parameters := make([]any, 0, len(expectedParameters))
			for _, parameterValue := range expectedParameters {
				reference := mustString(mustObject(parameterValue)["$ref"])
				name := strings.TrimPrefix(reference, "#/components/parameters/")
				component := mustObject(components[name])
				parameters = append(parameters, nativeParameterFixture(component))
			}
			responses := make(map[string]any)
			for status := range mustObject(expected["responses"]) {
				responses[status] = map[string]any{"description": "native registration"}
			}
			operation := map[string]any{
				"operationId": expected["operationId"],
				"parameters":  parameters,
				"responses":   responses,
				"security":    expected["security"],
			}
			operationID := mustString(expected["operationId"])
			if reference, present := nativeRequestSchemaByOperation[operationID]; present {
				if _, expectedBody := expected["requestBody"]; !expectedBody {
					panic("native request-body fixture disagrees with normalized operation " + operationID)
				}
				operation["requestBody"] = nativeRequestBodyFixture(reference)
			} else if _, expectedBody := expected["requestBody"]; expectedBody {
				panic("missing native request-body fixture for " + operationID)
			}
			operations[method] = operation
		}
		paths[path] = operations
	}
	return map[string]any{
		"info": map[string]any{
			"title":       "Echo Playground Portable API",
			"version":     "1.0.0",
			"description": "Portable REST contract implemented with Echo 5.3.",
		},
		"paths": paths,
	}
}

func nativeParameterFixture(component map[string]any) map[string]any {
	parameter := map[string]any{
		"description": "native registration",
		"name":        component["name"],
		"in":          component["in"],
	}
	if component["required"] == true {
		parameter["required"] = true
	}
	schema := make(map[string]any)
	for key, value := range mustObject(component["schema"]) {
		if key != "pattern" {
			schema[key] = value
		}
	}
	parameter["schema"] = schema
	return parameter
}

func nativeRequestBodyFixture(reference string) map[string]any {
	return map[string]any{
		"required": true,
		"content": map[string]any{
			"application/json": map[string]any{"schema": map[string]any{"oneOf": []any{
				map[string]any{"type": "object"},
				map[string]any{"$ref": reference},
			}}},
			"application/cbor": map[string]any{"schema": map[string]any{"type": "string"}},
		},
	}
}

func nativeOperation(t *testing.T, document map[string]any, path, method string) map[string]any {
	t.Helper()
	return mapValue(t, mapValue(t, mapValue(t, document, "paths"), path), method)
}

func walkReferences(t *testing.T, root, value any) {
	t.Helper()
	switch current := value.(type) {
	case map[string]any:
		if rawReference, ok := current["$ref"].(string); ok {
			if !strings.HasPrefix(rawReference, "#/") {
				t.Fatalf("non-local reference %q", rawReference)
			}
			resolved := root
			for escaped := range strings.SplitSeq(strings.TrimPrefix(rawReference, "#/"), "/") {
				part := strings.ReplaceAll(strings.ReplaceAll(escaped, "~1", "/"), "~0", "~")
				object, ok := resolved.(map[string]any)
				if !ok || object[part] == nil {
					t.Fatalf("unresolved reference %q", rawReference)
				}
				resolved = object[part]
			}
		}
		for _, child := range current {
			walkReferences(t, root, child)
		}
	case []any:
		for _, child := range current {
			walkReferences(t, root, child)
		}
	}
}

func mapValue(t *testing.T, parent map[string]any, key string) map[string]any {
	t.Helper()
	value, ok := parent[key].(map[string]any)
	if !ok {
		t.Fatalf("%s is not an object: %#v", key, parent[key])
	}
	return value
}

func objectValue(t *testing.T, value any, location string) map[string]any {
	t.Helper()
	object, ok := value.(map[string]any)
	if !ok {
		t.Fatalf("%s is not an object: %#v", location, value)
	}
	return object
}

func arrayValue(t *testing.T, value any, location string) []any {
	t.Helper()
	array, ok := value.([]any)
	if !ok {
		t.Fatalf("%s is not an array: %#v", location, value)
	}
	return array
}

func mustObject(value any) map[string]any {
	object, ok := value.(map[string]any)
	if !ok {
		panic(fmt.Sprintf("normalizer fixture value is not an object: %#v", value))
	}
	return object
}

func mustArray(value any) []any {
	array, ok := value.([]any)
	if !ok {
		panic(fmt.Sprintf("normalizer fixture value is not an array: %#v", value))
	}
	return array
}

func mustString(value any) string {
	text, ok := value.(string)
	if !ok {
		panic(fmt.Sprintf("normalizer fixture value is not a string: %#v", value))
	}
	return text
}

func assertMedia(t *testing.T, location string, content map[string]any, names ...string) {
	t.Helper()
	if len(content) != len(names) {
		t.Fatalf("%s media = %#v", location, content)
	}
	for _, name := range names {
		if content[name] == nil {
			t.Fatalf("%s lacks %s", location, name)
		}
	}
}

func assertStatusSet(t *testing.T, operation map[string]any, statuses ...string) {
	t.Helper()
	responses := mapValue(t, operation, "responses")
	if len(responses) != len(statuses) {
		t.Fatalf("responses = %#v, want %v", responses, statuses)
	}
	for _, status := range statuses {
		if responses[status] == nil {
			t.Fatalf("responses lack %s", status)
		}
	}
}
