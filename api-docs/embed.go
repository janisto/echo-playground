package apidocs

import _ "embed"

// OpenAPIJSON is the generated OpenAPI document served by the application.
//
//go:embed swagger.json
var OpenAPIJSON []byte
