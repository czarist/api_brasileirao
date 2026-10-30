// Package docs embeds the OpenAPI document and the Swagger UI assets.
package docs

import "embed"

// FS contains the contract and the two vendored Swagger UI files.
//
//go:embed openapi.yaml swagger.html assets/swagger-ui-bundle.js assets/swagger-ui.css
var FS embed.FS
