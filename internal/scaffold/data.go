package scaffold

import (
	"fmt"
	"strings"

	"github.com/esrid/gogen/internal/config"
)

// RefAssoc holds precomputed names for a foreign-key association.
// e.g. user:references on Post → ListPostsByUserID / ListByUserID
type RefAssoc struct {
	FieldGoName   string // "UserID"
	FieldName     string // "user_id"
	RefModel      string // "User"
	RefTable      string // "users"
	StoreMethod   string // "ListPostsByUserID"
	ServiceMethod string // "ListByUserID"
	ParamName     string // "userID"
	URLSegment    string // "post" — singular snake_case for route: /by-post/{postID}
	IsUserRef     bool   // true when RefTable == "users" (auth-scoped, no public route)
}

type Data struct {
	*config.ProjectConfig

	ModelName         string // "Post"
	ModelNameLC       string // "post"
	ModelNamePlural   string // "Posts"
	ModelNamePluralLC string // "posts"
	TableName         string // "posts"
	Fields            []Field
	Refs              []RefAssoc
	RoutePrefix       string // "/posts"

	// Precomputed SQL fragments
	InsertCols     string // "title, body, user_id"
	InsertArgs     string // "p.Title, p.Body, p.UserID"
	SQLiteInsert   string // "?, ?, ?"
	PGInsert       string // "$1, $2, $3"
	SelectCols     string // "id, title, body, user_id, created_at, updated_at"
	ScanArgs       string // "&p.ID, &p.Title, &p.Body, &p.UserID, &p.CreatedAt, &p.UpdatedAt"
	SQLiteUpdate   string // "title = ?, body = ?"
	PGUpdate       string // "title = $1, body = $2"
	UpdateArgs     string // "p.Title, p.Body, p.ID"
	PGUpdateWhereN string // "$3"

	HasTimeImport   bool
	NeedsStrconv    bool // any int or float field (needs strconv in SSR form parsing)
	HasNonUserRefs  bool // has at least one non-user references field

	Protected     bool   // --protected flag: routes require auth
	HasUserRef    bool   // has a user:references field
	UserRefGoName string // GoName of the user ref field, e.g. "UserID"
	UserRef       *RefAssoc // the user:references assoc (for handler template)
}

func NewData(modelName string, fields []Field, cfg *config.ProjectConfig) *Data {
	tableName := Pluralize(ToSnake(modelName))
	nameLC := strings.ToLower(modelName[:1]) + modelName[1:]

	d := &Data{
		ProjectConfig:     cfg,
		ModelName:         modelName,
		ModelNameLC:       nameLC,
		ModelNamePlural:   ToCamel(tableName),
		ModelNamePluralLC: tableName,
		TableName:         tableName,
		Fields:            fields,
		RoutePrefix:       "/" + tableName,
	}

	for _, f := range fields {
		if f.IsTime {
			d.HasTimeImport = true
		}
		if f.GoType == "int" || f.GoType == "float64" {
			d.NeedsStrconv = true
		}
		if f.IsRef {
			// derive singular model name from table: "users" → "User"
			singular := singularize(f.RefTable)
			refModel := ToCamel(singular)
			paramName := strings.ToLower(refModel[:1]) + refModel[1:] + "ID"

			isUserRef := f.RefTable == "users"
			assoc := RefAssoc{
				FieldGoName:   f.GoName,
				FieldName:     f.Name,
				RefModel:      refModel,
				RefTable:      f.RefTable,
				StoreMethod:   "List" + modelName + "sBy" + refModel + "ID",
				ServiceMethod: "ListBy" + refModel + "ID",
				ParamName:     paramName,
				URLSegment:    singularize(f.RefTable),
				IsUserRef:     isUserRef,
			}
			d.Refs = append(d.Refs, assoc)

			if isUserRef {
				d.HasUserRef = true
				d.UserRefGoName = f.GoName
				d.UserRef = &assoc
			} else {
				d.HasNonUserRefs = true
			}
		}
	}

	d.computeSQL()
	return d
}

func (d *Data) computeSQL() {
	var (
		insertCols, insertArgs []string
		sqliteInsert, pgInsert []string
		selectCols, scanArgs   []string
		sqliteUpdate, pgUpdate []string
		updateArgs             []string
	)

	selectCols = append(selectCols, "id")
	scanArgs = append(scanArgs, "&p.ID")

	for i, f := range d.Fields {
		insertCols = append(insertCols, f.Name)
		insertArgs = append(insertArgs, "p."+f.GoName)
		sqliteInsert = append(sqliteInsert, "?")
		pgInsert = append(pgInsert, fmt.Sprintf("$%d", i+1))

		selectCols = append(selectCols, f.Name)
		scanArgs = append(scanArgs, "&p."+f.GoName)
	}

	selectCols = append(selectCols, "created_at", "updated_at")
	scanArgs = append(scanArgs, "&p.CreatedAt", "&p.UpdatedAt")

	// UPDATE: non-ref fields only
	updateIdx := 1
	for _, f := range d.Fields {
		if f.IsRef {
			continue
		}
		sqliteUpdate = append(sqliteUpdate, f.Name+" = ?")
		pgUpdate = append(pgUpdate, fmt.Sprintf("%s = $%d", f.Name, updateIdx))
		updateArgs = append(updateArgs, "p."+f.GoName)
		updateIdx++
	}
	updateArgs = append(updateArgs, "p.ID")

	d.InsertCols = strings.Join(insertCols, ", ")
	d.InsertArgs = strings.Join(insertArgs, ", ")
	d.SQLiteInsert = strings.Join(sqliteInsert, ", ")
	d.PGInsert = strings.Join(pgInsert, ", ")
	d.SelectCols = strings.Join(selectCols, ", ")
	d.ScanArgs = strings.Join(scanArgs, ", ")
	d.SQLiteUpdate = strings.Join(sqliteUpdate, ", ")
	d.PGUpdate = strings.Join(pgUpdate, ", ")
	d.UpdateArgs = strings.Join(updateArgs, ", ")
	d.PGUpdateWhereN = fmt.Sprintf("$%d", updateIdx)
}

func singularize(s string) string {
	switch {
	case strings.HasSuffix(s, "ies"):
		return s[:len(s)-3] + "y"
	case strings.HasSuffix(s, "ses") || strings.HasSuffix(s, "xes") ||
		strings.HasSuffix(s, "zes") || strings.HasSuffix(s, "ches") ||
		strings.HasSuffix(s, "shes"):
		return s[:len(s)-2]
	case strings.HasSuffix(s, "s") && !strings.HasSuffix(s, "ss"):
		return s[:len(s)-1]
	}
	return s
}
