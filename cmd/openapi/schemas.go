package main

import (
	"maps"

	itemshttp "github.com/janisto/echo-playground/internal/http/v1/items"
	"github.com/janisto/echo-playground/internal/platform/timeutil"
)

const maximumSafeInteger = 9_007_199_254_740_991

func portableSchemas() map[string]any {
	schemas := map[string]any{
		"SafeInteger": safeIntegerSchema(),
		"Timestamp": map[string]any{
			"type":    "string",
			"format":  "date-time",
			"pattern": `^[0-9]{4}-(?:0[1-9]|1[0-2])-(?:0[1-9]|[12][0-9]|3[01])T(?:[01][0-9]|2[0-3]):[0-5][0-9]:[0-5][0-9]\.[0-9]{3}Z$`,
		},
		"OpaqueId":    map[string]any{"type": "string", "minLength": 1, "maxLength": 128},
		"BoundedName": boundedNameSchema(),
		"ContactEmail": map[string]any{
			"type":      "string",
			"minLength": 3,
			"maxLength": 254,
			"pattern":   `^(?=.{1,64}@)(?!\.)(?![^@]*\.\.)(?![^@]*\.@)[A-Za-z0-9!#$%&'*+/=?^_{|}~.-]+@[A-Za-z0-9](?:[A-Za-z0-9-]{0,61}[A-Za-z0-9])?(?:\.[A-Za-z0-9](?:[A-Za-z0-9-]{0,61}[A-Za-z0-9])?)+$`,
		},
		"ContactEmailInput": map[string]any{
			"type":    "string",
			"pattern": `^[\u0009-\u000D\u0020]*(?=[^\u0009-\u000D\u0020]{3,254}[\u0009-\u000D\u0020]*$)(?=[A-Za-z0-9!#$%&'*+/=?^_{|}~.-]{1,64}@)(?!\.)(?![^@]*\.\.)(?![^@]*\.@)[A-Za-z0-9!#$%&'*+/=?^_{|}~.-]+@[A-Za-z0-9](?:[A-Za-z0-9-]{0,61}[A-Za-z0-9])?(?:\.[A-Za-z0-9](?:[A-Za-z0-9-]{0,61}[A-Za-z0-9])?)+[\u0009-\u000D\u0020]*$`,
		},
		"PhoneNumber": map[string]any{"type": "string", "pattern": `^\+[1-9][0-9]{6,14}$`},
		"PhoneNumberInput": map[string]any{
			"type":    "string",
			"pattern": `^[\u0009-\u000D\u0020]*\+[1-9][0-9]{6,14}[\u0009-\u000D\u0020]*$`,
		},
		"ErrorSource": errorSourceSchema(),
		"ErrorDetail": closedObject(
			map[string]any{
				"detail": map[string]any{"type": "string", "minLength": 1, "maxLength": 200},
				"source": ref("schemas", "ErrorSource"),
			},
			"detail",
		),
		"Health": closedObject(map[string]any{
			"status": map[string]any{"type": "string", "const": "healthy"},
		}, "status"),
		"Greeting": closedObject(map[string]any{
			"message": map[string]any{"type": "string"},
		}, "message"),
		"HelloCreate": closedObject(map[string]any{
			"name": ref("schemas", "BoundedName"),
		}, "name"),
		"Money": closedObject(map[string]any{
			"amountMinor": ref("schemas", "SafeInteger"),
			"currency":    map[string]any{"type": "string", "const": "USD"},
		}, "amountMinor", "currency"),
	}
	schemas["Item"] = itemSchema()
	schemas["ItemPage"] = closedObject(map[string]any{
		"items": map[string]any{"type": "array", "maxItems": 100, "items": ref("schemas", "Item")},
		"total": ref("schemas", "SafeInteger"),
	}, "items", "total")
	schemas["ProfileCreate"] = profileCreateSchema()
	schemas["ProfileUpdate"] = profileUpdateSchema()
	schemas["Profile"] = profileSchema()
	for code, definition := range problemDefinitions() {
		schemas[problemSchemaName(code)] = problemSchema(code, definition)
	}
	maps.Copy(schemas, githubSchemas())
	return schemas
}

func safeIntegerSchema() map[string]any {
	return map[string]any{"type": "integer", "minimum": 0, "maximum": maximumSafeInteger}
}

func boundedNameSchema() map[string]any {
	return map[string]any{
		"type":      "string",
		"minLength": 1,
		"maxLength": 100,
		"pattern":   `^(?![\u0009-\u000D\u0020\u0085\u00A0\u1680\u2000-\u200A\u2028\u2029\u202F\u205F\u3000])(?!.*[\u0009-\u000D\u0020\u0085\u00A0\u1680\u2000-\u200A\u2028\u2029\u202F\u205F\u3000]$)[^\u0000-\u001F\u007F-\u009F]*$`,
	}
}

func itemSchema() map[string]any {
	catalog := itemshttp.Catalog()
	identifiers := make([]any, len(catalog))
	names := make([]any, len(catalog))
	timestamps := make([]any, len(catalog))
	descriptions := make([]any, len(catalog))
	exactItems := make([]any, len(catalog))
	for index, item := range catalog {
		identifiers[index] = item.ID
		names[index] = item.Name
		timestamp := item.CreatedAt.UTC().Format(timeutil.RFC3339Millis)
		timestamps[index] = timestamp
		descriptions[index] = item.Description
		exactItems[index] = map[string]any{
			"required": []any{"id", "name", "category", "price", "inStock", "createdAt", "description"},
			"properties": map[string]any{
				"id":          map[string]any{"const": item.ID},
				"name":        map[string]any{"const": item.Name},
				"category":    map[string]any{"const": item.Category},
				"inStock":     map[string]any{"const": item.InStock},
				"createdAt":   map[string]any{"const": timestamp},
				"description": map[string]any{"const": item.Description},
				"price": map[string]any{
					"required": []any{"amountMinor", "currency"},
					"properties": map[string]any{
						"amountMinor": map[string]any{"const": item.Price.AmountMinor},
						"currency":    map[string]any{"const": item.Price.Currency},
					},
				},
			},
		}
	}
	schema := closedObject(map[string]any{
		"id":   map[string]any{"type": "string", "enum": identifiers},
		"name": map[string]any{"type": "string", "enum": names},
		"category": map[string]any{
			"type": "string", "enum": []any{"electronics", "tools", "accessories", "robotics", "power", "components"},
		},
		"price":       ref("schemas", "Money"),
		"inStock":     map[string]any{"type": "boolean"},
		"createdAt":   map[string]any{"type": "string", "format": "date-time", "enum": timestamps},
		"description": map[string]any{"type": "string", "enum": descriptions},
	}, "id", "name", "category", "price", "inStock", "createdAt", "description")
	schema["allOf"] = []any{map[string]any{"oneOf": exactItems}}
	return schema
}

func profileCreateSchema() map[string]any {
	return closedObject(map[string]any{
		"firstName":      ref("schemas", "BoundedName"),
		"lastName":       ref("schemas", "BoundedName"),
		"contactEmail":   ref("schemas", "ContactEmailInput"),
		"phoneNumber":    ref("schemas", "PhoneNumberInput"),
		"marketingOptIn": map[string]any{"type": "boolean", "default": false},
		"termsAccepted":  map[string]any{"type": "boolean", "const": true},
	}, "firstName", "lastName", "contactEmail", "phoneNumber", "termsAccepted")
}

func profileUpdateSchema() map[string]any {
	schema := closedObject(map[string]any{
		"firstName":      ref("schemas", "BoundedName"),
		"lastName":       ref("schemas", "BoundedName"),
		"contactEmail":   ref("schemas", "ContactEmailInput"),
		"phoneNumber":    ref("schemas", "PhoneNumberInput"),
		"marketingOptIn": map[string]any{"type": "boolean"},
	})
	schema["minProperties"] = 1
	return schema
}

func profileSchema() map[string]any {
	return closedObject(map[string]any{
		"id":             ref("schemas", "OpaqueId"),
		"firstName":      ref("schemas", "BoundedName"),
		"lastName":       ref("schemas", "BoundedName"),
		"contactEmail":   ref("schemas", "ContactEmail"),
		"phoneNumber":    ref("schemas", "PhoneNumber"),
		"marketingOptIn": map[string]any{"type": "boolean"},
		"termsAccepted":  map[string]any{"type": "boolean", "const": true},
		"createdAt":      ref("schemas", "Timestamp"),
		"updatedAt":      ref("schemas", "Timestamp"),
	}, "id", "firstName", "lastName", "contactEmail", "phoneNumber", "marketingOptIn", "termsAccepted", "createdAt", "updatedAt")
}

type problemDefinition struct {
	status int
	title  string
	detail string
}

func problemDefinitions() map[string]problemDefinition {
	return map[string]problemDefinition{
		"invalid_request":        {400, "Bad Request", "Request is malformed"},
		"unauthorized":           {401, "Unauthorized", "Authentication is required or invalid"},
		"forbidden":              {403, "Forbidden", "Access is forbidden"},
		"not_found":              {404, "Not Found", "Resource not found"},
		"profile_not_found":      {404, "Not Found", "Profile not found"},
		"github_not_found":       {404, "Not Found", "GitHub resource not found"},
		"not_acceptable":         {406, "Not Acceptable", "No acceptable response representation is available"},
		"profile_exists":         {409, "Conflict", "Profile already exists"},
		"payload_too_large":      {413, "Content Too Large", "Request body is too large"},
		"unsupported_media_type": {415, "Unsupported Media Type", "Request representation is not supported"},
		"validation_failed":      {422, "Unprocessable Content", "Request validation failed"},
		"github_rate_limit":      {429, "Too Many Requests", "GitHub rate limit exceeded"},
		"internal_error":         {500, "Internal Server Error", "Internal server error"},
		"github_upstream":        {502, "Bad Gateway", "GitHub upstream response is invalid or unavailable"},
		"dependency_unavailable": {503, "Service Unavailable", "A required dependency is unavailable"},
		"github_timeout":         {504, "Gateway Timeout", "GitHub request timed out"},
	}
}

func problemSchema(code string, definition problemDefinition) map[string]any {
	return closedObject(map[string]any{
		"type":   map[string]any{"type": "string", "const": "about:blank"},
		"title":  map[string]any{"type": "string", "const": definition.title},
		"status": map[string]any{"type": "integer", "const": definition.status},
		"detail": map[string]any{"type": "string", "const": definition.detail},
		"code":   map[string]any{"type": "string", "const": code},
		"errors": map[string]any{
			"type":     "array",
			"minItems": 1,
			"maxItems": 32,
			"items":    ref("schemas", "ErrorDetail"),
		},
	}, "title", "status", "detail", "code")
}

func errorSourceSchema() map[string]any {
	return map[string]any{"oneOf": []any{
		closedObject(
			map[string]any{"pointer": map[string]any{"type": "string", "minLength": 1, "maxLength": 256}},
			"pointer",
		),
		closedObject(
			map[string]any{"parameter": map[string]any{"type": "string", "minLength": 1, "maxLength": 256}},
			"parameter",
		),
		closedObject(
			map[string]any{"header": map[string]any{"type": "string", "minLength": 1, "maxLength": 256}},
			"header",
		),
	}}
}

func githubSchemas() map[string]any {
	nullableString := func() map[string]any {
		return map[string]any{"type": []any{"string", "null"}, "minLength": 1}
	}
	nullableURI := func() map[string]any {
		return map[string]any{"oneOf": []any{absoluteURI(), map[string]any{"type": "null"}}}
	}
	nullableLicense := func() map[string]any {
		return map[string]any{
			"type":      []any{"string", "null"},
			"minLength": 1,
			"not":       map[string]any{"const": "NOASSERTION"},
		}
	}
	repositorySummaryProperties := func() map[string]any {
		return map[string]any{
			"id": ref("schemas", "SafeInteger"), "name": nonEmptyString(), "fullName": nonEmptyString(),
			"description": nullableString(), "htmlUrl": absoluteURI(), "fork": map[string]any{"type": "boolean"},
		}
	}
	schemas := map[string]any{}
	schemas["GitHubOwner"] = closedObject(map[string]any{
		"id":          ref("schemas", "SafeInteger"),
		"login":       nonEmptyString(),
		"type":        nonEmptyString(),
		"name":        nullableString(),
		"avatarUrl":   absoluteURI(),
		"htmlUrl":     absoluteURI(),
		"company":     nullableString(),
		"blog":        nullableString(),
		"location":    nullableString(),
		"bio":         nullableString(),
		"publicRepos": ref("schemas", "SafeInteger"),
		"followers":   ref("schemas", "SafeInteger"),
		"following": ref(
			"schemas",
			"SafeInteger",
		),
		"createdAt": ref("schemas", "Timestamp"),
		"updatedAt": ref("schemas", "Timestamp"),
	}, "id", "login", "type", "name", "avatarUrl", "htmlUrl", "company", "blog", "location", "bio", "publicRepos", "followers", "following", "createdAt", "updatedAt")
	schemas["GitHubRepositorySummary"] = closedObject(
		repositorySummaryProperties(),
		"id",
		"name",
		"fullName",
		"description",
		"htmlUrl",
		"fork",
	)
	repositoryProperties := repositorySummaryProperties()
	repositoryDetailProperties := map[string]any{
		"language":        nullableString(),
		"stargazersCount": ref("schemas", "SafeInteger"),
		"forksCount":      ref("schemas", "SafeInteger"),
		"openIssuesCount": ref("schemas", "SafeInteger"),
		"archived":        map[string]any{"type": "boolean"},
		"createdAt":       ref("schemas", "Timestamp"),
		"updatedAt":       ref("schemas", "Timestamp"),
		"pushedAt":        map[string]any{"oneOf": []any{ref("schemas", "Timestamp"), map[string]any{"type": "null"}}},
		"defaultBranch":   nonEmptyString(),
		"license":         nullableLicense(),
		"topics": map[string]any{
			"type":        "array",
			"uniqueItems": true,
			"items":       map[string]any{"type": "string"},
		},
		"disabled": map[string]any{"type": "boolean"},
	}
	maps.Copy(repositoryProperties, repositoryDetailProperties)
	schemas["GitHubRepository"] = closedObject(
		repositoryProperties,
		"id",
		"name",
		"fullName",
		"description",
		"htmlUrl",
		"fork",
		"language",
		"stargazersCount",
		"forksCount",
		"openIssuesCount",
		"archived",
		"createdAt",
		"updatedAt",
		"pushedAt",
		"defaultBranch",
		"license",
		"topics",
		"disabled",
	)
	activity := closedObject(map[string]any{
		"id": ref("schemas", "SafeInteger"), "actor": nullableString(), "actorAvatarUrl": nullableURI(),
		"ref": nonEmptyString(), "timestamp": ref("schemas", "Timestamp"), "activityType": nonEmptyString(),
	}, "id", "actor", "actorAvatarUrl", "ref", "timestamp", "activityType")
	activity["allOf"] = []any{
		map[string]any{
			"if": map[string]any{
				"properties": map[string]any{"actor": map[string]any{"type": "null"}},
				"required":   []any{"actor"},
			},
			"then": map[string]any{"properties": map[string]any{"actorAvatarUrl": map[string]any{"type": "null"}}},
			"else": map[string]any{"properties": map[string]any{"actorAvatarUrl": absoluteURI()}},
		},
	}
	schemas["GitHubActivity"] = activity
	schemas["GitHubLanguage"] = closedObject(
		map[string]any{"name": nonEmptyString(), "bytes": ref("schemas", "SafeInteger")},
		"name",
		"bytes",
	)
	schemas["GitHubTagCommit"] = closedObject(map[string]any{
		"sha": map[string]any{"type": "string", "pattern": `^(?:[0-9a-f]{40}|[0-9a-f]{64})$`},
	}, "sha")
	schemas["GitHubTag"] = closedObject(
		map[string]any{"name": nonEmptyString(), "commit": ref("schemas", "GitHubTagCommit")},
		"name",
		"commit",
	)
	schemas["GitHubRepositoryPage"] = closedObject(map[string]any{
		"repos": map[string]any{"type": "array", "maxItems": 100, "items": ref("schemas", "GitHubRepositorySummary")},
		"count": map[string]any{"type": "integer", "minimum": 0, "maximum": 100},
	}, "repos", "count")
	schemas["GitHubActivityPage"] = closedObject(map[string]any{
		"activities": map[string]any{"type": "array", "maxItems": 100, "items": ref("schemas", "GitHubActivity")},
		"count":      map[string]any{"type": "integer", "minimum": 0, "maximum": 100},
	}, "activities", "count")
	schemas["GitHubLanguages"] = closedObject(map[string]any{
		"languages": map[string]any{"type": "array", "items": ref("schemas", "GitHubLanguage")},
	}, "languages")
	schemas["GitHubTagPage"] = closedObject(map[string]any{
		"tags":  map[string]any{"type": "array", "maxItems": 100, "items": ref("schemas", "GitHubTag")},
		"count": map[string]any{"type": "integer", "minimum": 0, "maximum": 100},
	}, "tags", "count")
	return schemas
}

func closedObject(properties map[string]any, required ...string) map[string]any {
	schema := map[string]any{"type": "object", "additionalProperties": false, "properties": properties}
	if len(required) > 0 {
		values := make([]any, len(required))
		for index, name := range required {
			values[index] = name
		}
		schema["required"] = values
	}
	return schema
}

func nonEmptyString() map[string]any { return map[string]any{"type": "string", "minLength": 1} }

func absoluteURI() map[string]any {
	return map[string]any{"type": "string", "format": "uri", "pattern": `^https?://`}
}

func problemSchemaName(code string) string { return "Problem_" + code }
