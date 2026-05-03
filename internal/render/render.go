package render

import (
	"bytes"
	"go/format"
	"io/fs"
	"strings"
	"text/template"

	"github.com/esrid/gogen/templates"
)

// File renders a template file with the given data.
// Files with .tmpl extension are rendered through text/template with [[ ]] delimiters.
// Other files (HTML, SQL, etc.) are copied verbatim.
// .go.tmpl files are additionally formatted with go/format.
func File(templatePath string, data any) ([]byte, error) {
	content, err := fs.ReadFile(templates.FS, templatePath)
	if err != nil {
		return nil, err
	}

	if !strings.HasSuffix(templatePath, ".tmpl") {
		return content, nil
	}

	tmpl, err := template.New(templatePath).Delims("[[", "]]").Parse(string(content))
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, err
	}

	result := buf.Bytes()

	stripped := strings.TrimSuffix(templatePath, ".tmpl")
	if strings.HasSuffix(stripped, ".go") {
		if formatted, fmtErr := format.Source(result); fmtErr == nil {
			return formatted, nil
		}
	}

	return result, nil
}
