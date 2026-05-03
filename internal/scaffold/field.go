package scaffold

import (
	"fmt"
	"strings"
)

type Field struct {
	Name      string // snake_case: "title", "user_id"
	GoName    string // CamelCase: "Title", "UserID"
	GoType    string // "string", "int", "bool", "float64", "time.Time"
	JSONTag   string // "title", "user_id"
	SQLiteCol string // SQLite column definition
	PGCol     string // Postgres column definition
	IsRef     bool
	RefTable  string // "users"
	IsTime    bool
}

func ParseFields(args []string) ([]Field, error) {
	var fields []Field
	for _, arg := range args {
		f, err := parseField(arg)
		if err != nil {
			return nil, err
		}
		fields = append(fields, f)
	}
	return fields, nil
}

func parseField(arg string) (Field, error) {
	parts := strings.SplitN(arg, ":", 2)
	if len(parts) != 2 {
		return Field{}, fmt.Errorf("invalid field %q: use name:type", arg)
	}
	name := strings.ToLower(parts[0])
	typ := strings.ToLower(parts[1])

	f := Field{
		Name:    name,
		GoName:  ToCamel(name),
		JSONTag: name,
	}

	switch typ {
	case "string", "text":
		f.GoType = "string"
		f.SQLiteCol = "TEXT NOT NULL DEFAULT ''"
		f.PGCol = "TEXT NOT NULL DEFAULT ''"
	case "int", "integer":
		f.GoType = "int"
		f.SQLiteCol = "INTEGER NOT NULL DEFAULT 0"
		f.PGCol = "INTEGER NOT NULL DEFAULT 0"
	case "bool", "boolean":
		f.GoType = "bool"
		f.SQLiteCol = "INTEGER NOT NULL DEFAULT 0"
		f.PGCol = "BOOLEAN NOT NULL DEFAULT false"
	case "float", "decimal", "numeric":
		f.GoType = "float64"
		f.SQLiteCol = "REAL NOT NULL DEFAULT 0"
		f.PGCol = "NUMERIC NOT NULL DEFAULT 0"
	case "time", "datetime", "timestamp":
		f.GoType = "time.Time"
		f.SQLiteCol = "DATETIME"
		f.PGCol = "TIMESTAMPTZ"
		f.IsTime = true
	case "uuid":
		f.GoType = "string"
		f.SQLiteCol = "TEXT NOT NULL DEFAULT ''"
		f.PGCol = "UUID NOT NULL DEFAULT gen_random_uuid()"
	case "references":
		refTable := Pluralize(name)
		f.Name = name + "_id"
		f.GoName = ToCamel(name + "_id")
		f.JSONTag = name + "_id"
		f.GoType = "string"
		f.SQLiteCol = "TEXT NOT NULL REFERENCES " + refTable + "(id) ON DELETE CASCADE"
		f.PGCol = "UUID NOT NULL REFERENCES " + refTable + "(id) ON DELETE CASCADE"
		f.IsRef = true
		f.RefTable = refTable
	default:
		return Field{}, fmt.Errorf("unknown type %q (valid: string, text, int, bool, float, time, uuid, references)", typ)
	}

	return f, nil
}

var acronyms = map[string]string{
	"id":   "ID",
	"url":  "URL",
	"uri":  "URI",
	"api":  "API",
	"http": "HTTP",
	"sql":  "SQL",
	"db":   "DB",
	"ip":   "IP",
	"uuid": "UUID",
}

func ToCamel(s string) string {
	parts := strings.Split(s, "_")
	for i, p := range parts {
		if len(p) == 0 {
			continue
		}
		if up, ok := acronyms[strings.ToLower(p)]; ok {
			parts[i] = up
		} else {
			parts[i] = strings.ToUpper(p[:1]) + p[1:]
		}
	}
	return strings.Join(parts, "")
}

func Pluralize(s string) string {
	s = strings.ToLower(s)
	for _, suffix := range []string{"sh", "ch", "ss", "x", "z"} {
		if strings.HasSuffix(s, suffix) {
			return s + "es"
		}
	}
	if strings.HasSuffix(s, "y") && len(s) > 1 {
		if !strings.ContainsRune("aeiou", rune(s[len(s)-2])) {
			return s[:len(s)-1] + "ies"
		}
	}
	if strings.HasSuffix(s, "s") {
		return s
	}
	return s + "s"
}

func ToSnake(s string) string {
	var b strings.Builder
	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			b.WriteByte('_')
		}
		b.WriteRune(r)
	}
	return strings.ToLower(b.String())
}
