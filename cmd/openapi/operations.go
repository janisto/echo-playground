package main

import "strconv"

func portablePaths() map[string]any {
	paths := map[string]any{}
	paths["/health"] = map[string]any{"get": publicOperation(
		"getHealth",
		"Get application health",
		nil,
		nil,
		responses(
			success("Health"),
			failure(400, "invalid_request"),
			failure(406, "not_acceptable"),
			failure(500, "internal_error"),
		),
	)}
	paths["/v1/hello"] = map[string]any{
		"get": publicOperation(
			"getHello",
			"Get the default greeting",
			nil,
			nil,
			responses(
				success("Greeting"),
				failure(400, "invalid_request"),
				failure(406, "not_acceptable"),
				failure(500, "internal_error"),
			),
		),
		"post": publicOperation(
			"createHello", "Create a personalized greeting", nil, requestBody("HelloCreate"),
			responses(
				success("Greeting"),
				failure(400, "invalid_request"),
				failure(406, "not_acceptable"),
				failure(
					413,
					"payload_too_large",
				),
				failure(415, "unsupported_media_type"),
				failure(422, "validation_failed"),
				failure(500, "internal_error"),
			),
		),
	}
	paths["/v1/items"] = map[string]any{"get": publicOperation(
		"listItems",
		"List the fixed item catalog",
		[]any{parameterRef("Limit"), parameterRef("Cursor"), parameterRef("Category")},
		nil,
		responses(
			successWithHeaders(200, "ItemPage", "Link"),
			failure(400, "invalid_request"),
			failure(406, "not_acceptable"),
			failure(422, "validation_failed"),
			failure(500, "internal_error"),
		),
	)}
	paths["/v1/profile"] = map[string]any{
		"post": protectedOperation(
			"createProfile", "Create the current principal profile", requestBody("ProfileCreate"),
			responses(
				successWithHeaders(201, "Profile", "Location"),
				authFailure(),
				failure(400, "invalid_request"),
				failure(406, "not_acceptable"),
				failure(409, "profile_exists"),
				failure(413, "payload_too_large"),
				failure(
					415,
					"unsupported_media_type",
				),
				failure(422, "validation_failed"),
				failure(500, "internal_error"),
				failure(503, "dependency_unavailable"),
			),
		),
		"get": protectedOperation(
			"getProfile",
			"Get the current principal profile",
			nil,
			responses(
				success("Profile"),
				authFailure(),
				failure(400, "invalid_request"),
				failure(404, "profile_not_found"),
				failure(406, "not_acceptable"),
				failure(500, "internal_error"),
				failure(503, "dependency_unavailable"),
			),
		),
		"patch": protectedOperation(
			"updateProfile",
			"Update the current principal profile",
			requestBody("ProfileUpdate"),
			responses(
				success("Profile"),
				authFailure(),
				failure(400, "invalid_request"),
				failure(404, "profile_not_found"),
				failure(
					406,
					"not_acceptable",
				),
				failure(413, "payload_too_large"),
				failure(415, "unsupported_media_type"),
				failure(
					422,
					"validation_failed",
				),
				failure(500, "internal_error"),
				failure(503, "dependency_unavailable"),
			),
		),
		"delete": protectedOperation(
			"deleteProfile",
			"Delete the current principal profile",
			nil,
			responses(
				emptySuccess(204),
				authFailure(),
				failure(400, "invalid_request"),
				failure(404, "profile_not_found"),
				failure(500, "internal_error"),
				failure(503, "dependency_unavailable"),
			),
		),
	}
	addGitHubPaths(paths)
	return paths
}

func addGitHubPaths(paths map[string]any) {
	owner := parameterRef("Owner")
	repository := parameterRef("Repository")
	pointFailures := func() []responseSpec {
		return []responseSpec{
			failure(400, "invalid_request"),
			failure(404, "github_not_found"),
			failure(406, "not_acceptable"),
			failure(
				422,
				"validation_failed",
			),
			githubRateFailure(),
			failure(500, "internal_error"),
			failure(502, "github_upstream"),
			failure(504, "github_timeout"),
		}
	}
	githubResponses := func(schema string, paginated bool) map[string]any {
		specifications := []responseSpec{success(schema)}
		if paginated {
			specifications[0] = successWithHeaders(200, schema, "Link")
		}
		return responses(append(specifications, pointFailures()...)...)
	}
	paths["/v1/github/owners/{owner}"] = map[string]any{"get": publicOperation(
		"getGitHubOwner", "Get a public GitHub owner", []any{owner}, nil, githubResponses("GitHubOwner", false),
	)}
	paths["/v1/github/owners/{owner}/repos"] = map[string]any{"get": publicOperation(
		"listGitHubOwnerRepositories", "List public GitHub owner repositories",
		[]any{owner, parameterRef("Limit"), parameterRef("Cursor")}, nil, githubResponses("GitHubRepositoryPage", true),
	)}
	paths["/v1/github/repos/{owner}/{repo}"] = map[string]any{"get": publicOperation(
		"getGitHubRepository",
		"Get a public GitHub repository",
		[]any{owner, repository},
		nil,
		githubResponses("GitHubRepository", false),
	)}
	paths["/v1/github/repos/{owner}/{repo}/activity"] = map[string]any{"get": publicOperation(
		"listGitHubRepositoryActivity",
		"List public GitHub repository activity",
		[]any{
			owner,
			repository,
			parameterRef("Limit"),
			parameterRef("Cursor"),
		},
		nil,
		githubResponses("GitHubActivityPage", true),
	)}
	paths["/v1/github/repos/{owner}/{repo}/languages"] = map[string]any{"get": publicOperation(
		"listGitHubRepositoryLanguages", "List public GitHub repository languages",
		[]any{owner, repository}, nil, githubResponses("GitHubLanguages", false),
	)}
	paths["/v1/github/repos/{owner}/{repo}/tags"] = map[string]any{"get": publicOperation(
		"listGitHubRepositoryTags",
		"List public GitHub repository tags",
		[]any{
			owner,
			repository,
			parameterRef("Limit"),
			parameterRef("Cursor"),
		},
		nil,
		githubResponses("GitHubTagPage", true),
	)}
}

func publicOperation(id, summary string, parameters []any, body, operationResponses map[string]any) map[string]any {
	return operation(id, summary, []any{}, parameters, body, operationResponses)
}

func protectedOperation(id, summary string, body, operationResponses map[string]any) map[string]any {
	return operation(id, summary, []any{map[string]any{"BearerAuth": []any{}}}, nil, body, operationResponses)
}

func operation(
	id, summary string,
	security []any,
	parameters []any,
	body map[string]any,
	operationResponses map[string]any,
) map[string]any {
	allParameters := []any{parameterRef("XRequestID")}
	allParameters = append(allParameters, parameters...)
	result := map[string]any{
		"operationId": id, "summary": summary, "security": security,
		"description": "The query string is closed; unknown, repeated, or malformed parameters are rejected.",
		"parameters":  allParameters, "responses": operationResponses,
	}
	if body != nil {
		result["requestBody"] = body
	}
	return result
}

func requestBody(schema string) map[string]any {
	return map[string]any{
		"required": true,
		"content": map[string]any{
			"application/json": map[string]any{"schema": ref("schemas", schema)},
			"application/cbor": map[string]any{"schema": ref("schemas", schema)},
		},
	}
}

type responseSpec struct {
	status   int
	response map[string]any
}

func success(schema string) responseSpec { return successWithHeaders(200, schema) }

func successWithHeaders(status int, schema string, extraHeaders ...string) responseSpec {
	return responseSpec{status: status, response: map[string]any{
		"description": "Successful response",
		"headers":     responseHeaders(extraHeaders...),
		"content": map[string]any{
			"application/json": map[string]any{"schema": ref("schemas", schema)},
			"application/cbor": map[string]any{"schema": ref("schemas", schema)},
		},
	}}
}

func emptySuccess(status int) responseSpec {
	return responseSpec{status: status, response: map[string]any{
		"description": "Successful response with no content",
		"headers":     responseHeaders(),
	}}
}

func failure(status int, code string) responseSpec { return failureWithHeaders(status, code) }

func failureWithHeaders(status int, code string, extraHeaders ...string) responseSpec {
	return responseSpec{status: status, response: map[string]any{
		"description": "Controlled " + code + " failure",
		"headers":     responseHeaders(extraHeaders...),
		"content": map[string]any{
			"application/problem+json": map[string]any{"schema": ref("schemas", problemSchemaName(code))},
			"application/cbor":         map[string]any{"schema": ref("schemas", problemSchemaName(code))},
		},
	}}
}

func authFailure() responseSpec { return failureWithHeaders(401, "unauthorized", "WWWAuthenticate") }

func githubRateFailure() responseSpec {
	return failureWithHeaders(429, "github_rate_limit", "RetryAfter", "RateLimitReset")
}

func responses(specifications ...responseSpec) map[string]any {
	result := make(map[string]any, len(specifications))
	for _, specification := range specifications {
		result[decimalStatus(specification.status)] = specification.response
	}
	return result
}

func responseHeaders(extra ...string) map[string]any {
	headers := map[string]any{
		"X-Request-ID":           headerRef("XRequestID"),
		"Cache-Control":          headerRef("CacheControl"),
		"X-Content-Type-Options": headerRef("ContentTypeOptions"),
		"X-Frame-Options":        headerRef("FrameOptions"),
		"Referrer-Policy":        headerRef("ReferrerPolicy"),
		"Vary":                   headerRef("Vary"),
	}
	for _, name := range extra {
		switch name {
		case "Link":
			headers["Link"] = headerRef("Link")
		case "Location":
			headers["Location"] = headerRef("Location")
		case "WWWAuthenticate":
			headers["WWW-Authenticate"] = headerRef("WWWAuthenticate")
		case "RetryAfter":
			headers["Retry-After"] = headerRef("RetryAfter")
		case "RateLimitReset":
			headers["X-RateLimit-Reset"] = headerRef("RateLimitReset")
		}
	}
	return headers
}

func parameterComponents() map[string]any {
	return map[string]any{
		"XRequestID": map[string]any{
			"name":        "X-Request-ID",
			"in":          "header",
			"required":    false,
			"description": "A missing, invalid, repeated, or comma-combined candidate is replaced with a generated request ID.",
			"schema": map[string]any{
				"type":      "string",
				"minLength": 1,
				"maxLength": 128,
				"pattern":   `^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$`,
			},
		},
		"Owner": map[string]any{
			"name":     "owner",
			"in":       "path",
			"required": true,
			"schema": map[string]any{
				"type":      "string",
				"minLength": 1,
				"maxLength": 39,
				"pattern":   `^[A-Za-z0-9](?:[A-Za-z0-9_-]{0,37}[A-Za-z0-9])?$`,
			},
		},
		"Repository": map[string]any{
			"name":     "repo",
			"in":       "path",
			"required": true,
			"schema": map[string]any{
				"type":      "string",
				"minLength": 1,
				"maxLength": 100,
				"pattern":   `^(?=.*[A-Za-z0-9_-])[A-Za-z0-9._-]+$`,
			},
		},
		"Limit": map[string]any{
			"name": "limit", "in": "query", "required": false,
			"description": "Closed scalar query; unknown or repeated parameters are rejected.",
			"schema":      map[string]any{"type": "integer", "minimum": 1, "maximum": 100, "default": 20},
		},
		"Cursor": map[string]any{
			"name": "cursor", "in": "query", "required": false,
			"description": "Opaque scoped cursor in the closed scalar query.",
			"schema":      map[string]any{"type": "string", "minLength": 1, "maxLength": 2048, "pattern": `^[!-~]+$`},
		},
		"Category": map[string]any{
			"name":        "category",
			"in":          "query",
			"required":    false,
			"description": "Exact category filter in the closed scalar query.",
			"schema": map[string]any{
				"type": "string",
				"enum": []any{"electronics", "tools", "accessories", "robotics", "power", "components"},
			},
		},
	}
}

func headerComponents() map[string]any {
	return map[string]any{
		"XRequestID": map[string]any{
			"description": "Selected immutable request correlation value.",
			"schema": map[string]any{
				"type":      "string",
				"minLength": 1,
				"maxLength": 128,
				"pattern":   `^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$`,
			},
		},
		"CacheControl":       literalHeader("no-store"),
		"ContentTypeOptions": literalHeader("nosniff"),
		"FrameOptions":       literalHeader("DENY"),
		"ReferrerPolicy":     literalHeader("strict-origin-when-cross-origin"),
		"Vary": map[string]any{
			"description": "Response selection variance.",
			"schema":      map[string]any{"type": "string"},
		},
		"Link": map[string]any{
			"description": "RFC 8288 relative next and previous navigation.",
			"schema":      map[string]any{"type": "string"},
		},
		"Location":        literalHeader("/v1/profile"),
		"WWWAuthenticate": literalHeader("Bearer"),
		"RetryAfter": map[string]any{
			"description": "Nonnegative safe-integer delay in seconds.",
			"schema":      safeIntegerSchema(),
		},
		"RateLimitReset": map[string]any{
			"description": "Optional validated nonnegative safe-integer reset epoch.",
			"schema":      safeIntegerSchema(),
		},
	}
}

func securitySchemes() map[string]any {
	//nolint:gosec // BearerAuth is an OpenAPI scheme identifier, not a credential.
	return map[string]any{"BearerAuth": map[string]any{
		"type": "http", "scheme": "bearer", "bearerFormat": "Firebase ID token",
		"description": "Firebase ID token verified with revocation checking.",
	}}
}

func literalHeader(value string) map[string]any {
	return map[string]any{"schema": map[string]any{"type": "string", "const": value}}
}

func parameterRef(name string) map[string]any { return ref("parameters", name) }
func headerRef(name string) map[string]any    { return ref("headers", name) }

func decimalStatus(status int) string {
	return strconv.Itoa(status)
}
