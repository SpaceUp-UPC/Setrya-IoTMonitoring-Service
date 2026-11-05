// Package api embebe la especificación OpenAPI del servicio.
package api

import _ "embed"

//go:embed openapi.yaml
var OpenAPI []byte
