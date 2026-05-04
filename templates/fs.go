package templates

import "embed"

//go:embed all:new all:scaffold all:controller
var FS embed.FS
