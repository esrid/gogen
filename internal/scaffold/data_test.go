package scaffold

import (
	"strings"
	"testing"

	"github.com/esrid/gogen/internal/config"
)

// baseCfg returns a minimal ProjectConfig suitable for test use.
func baseCfg() *config.ProjectConfig {
	return &config.ProjectConfig{
		ProjectName: "myapp",
		ModulePath:  "github.com/example/myapp",
		DB:          "sqlite",
		RenderMode:  "api",
	}
}

// --------------------------------------------------------------------------
// ParseFields
// --------------------------------------------------------------------------

func TestParseFields_ValidTypes(t *testing.T) {
	cases := []struct {
		input   string
		name    string
		goName  string
		goType  string
		isRef   bool
		isTime  bool
	}{
		{"title:string", "title", "Title", "string", false, false},
		{"user:references", "user_id", "UserID", "string", true, false},
		{"views:int", "views", "Views", "int", false, false},
		{"price:float", "price", "Price", "float64", false, false},
		{"active:bool", "active", "Active", "bool", false, false},
		{"created:time", "created", "Created", "time.Time", false, true},
		{"uid:uuid", "uid", "Uid", "string", false, false},
	}

	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			fields, err := ParseFields([]string{tc.input})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(fields) != 1 {
				t.Fatalf("expected 1 field, got %d", len(fields))
			}
			f := fields[0]
			if f.Name != tc.name {
				t.Errorf("Name: want %q, got %q", tc.name, f.Name)
			}
			if f.GoName != tc.goName {
				t.Errorf("GoName: want %q, got %q", tc.goName, f.GoName)
			}
			if f.GoType != tc.goType {
				t.Errorf("GoType: want %q, got %q", tc.goType, f.GoType)
			}
			if f.IsRef != tc.isRef {
				t.Errorf("IsRef: want %v, got %v", tc.isRef, f.IsRef)
			}
			if f.IsTime != tc.isTime {
				t.Errorf("IsTime: want %v, got %v", tc.isTime, f.IsTime)
			}
		})
	}
}

func TestParseFields_ReferencesRefTable(t *testing.T) {
	fields, err := ParseFields([]string{"user:references"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fields[0].RefTable != "users" {
		t.Errorf("RefTable: want %q, got %q", "users", fields[0].RefTable)
	}
}

func TestParseFields_AliasedReference(t *testing.T) {
	fields, err := ParseFields([]string{"manager:user:references"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	f := fields[0]
	if f.Name != "manager_id" {
		t.Errorf("Name: want manager_id, got %q", f.Name)
	}
	if f.GoName != "ManagerID" {
		t.Errorf("GoName: want ManagerID, got %q", f.GoName)
	}
	if f.RefTable != "users" {
		t.Errorf("RefTable: want users, got %q", f.RefTable)
	}
	if !f.IsRef {
		t.Error("IsRef should be true")
	}
}

func TestParseFields_Errors(t *testing.T) {
	cases := []struct {
		input string
		desc  string
	}{
		{"title", "missing colon"},
		{"title:widget", "unknown type"},
		{"", "empty string"},
		{"a:b:string", "3-part non-references"},
		{"a:b:c:references", "4-part"},
	}

	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			_, err := ParseFields([]string{tc.input})
			if err == nil {
				t.Errorf("expected error for %q, got nil", tc.input)
			}
		})
	}
}

// --------------------------------------------------------------------------
// NewData — no fields
// --------------------------------------------------------------------------

func TestNewData_NoFields(t *testing.T) {
	d := NewData("Post", nil, baseCfg())

	if d.ModelName != "Post" {
		t.Errorf("ModelName: want Post, got %q", d.ModelName)
	}
	if d.ModelNameLC != "post" {
		t.Errorf("ModelNameLC: want post, got %q", d.ModelNameLC)
	}
	if d.TableName != "posts" {
		t.Errorf("TableName: want posts, got %q", d.TableName)
	}
	if d.RoutePrefix != "/posts" {
		t.Errorf("RoutePrefix: want /posts, got %q", d.RoutePrefix)
	}
	if d.ModelNamePlural != "Posts" {
		t.Errorf("ModelNamePlural: want Posts, got %q", d.ModelNamePlural)
	}
	if d.HasUserRef {
		t.Error("HasUserRef should be false with no fields")
	}
	if d.HasNonUserRefs {
		t.Error("HasNonUserRefs should be false with no fields")
	}
}

// --------------------------------------------------------------------------
// NewData — user:references field
// --------------------------------------------------------------------------

func TestNewData_WithUserRef(t *testing.T) {
	fields, err := ParseFields([]string{"user:references"})
	if err != nil {
		t.Fatalf("ParseFields: %v", err)
	}
	d := NewData("Post", fields, baseCfg())

	if !d.HasUserRef {
		t.Error("HasUserRef should be true")
	}
	if d.HasNonUserRefs {
		t.Error("HasNonUserRefs should be false")
	}
	if d.UserRefGoName != "UserID" {
		t.Errorf("UserRefGoName: want UserID, got %q", d.UserRefGoName)
	}
	if d.UserRef == nil {
		t.Fatal("UserRef should not be nil")
	}
	if !d.UserRef.IsUserRef {
		t.Error("UserRef.IsUserRef should be true")
	}
	if len(d.Refs) != 1 {
		t.Errorf("Refs length: want 1, got %d", len(d.Refs))
	}
	ref := d.Refs[0]
	if ref.RefModel != "User" {
		t.Errorf("RefModel: want User, got %q", ref.RefModel)
	}
}

// --------------------------------------------------------------------------
// NewData — post:references (non-user ref)
// --------------------------------------------------------------------------

func TestNewData_WithNonUserRef(t *testing.T) {
	fields, err := ParseFields([]string{"category:references"})
	if err != nil {
		t.Fatalf("ParseFields: %v", err)
	}
	d := NewData("Post", fields, baseCfg())

	if d.HasUserRef {
		t.Error("HasUserRef should be false")
	}
	if !d.HasNonUserRefs {
		t.Error("HasNonUserRefs should be true")
	}
	if len(d.Refs) != 1 {
		t.Fatalf("Refs length: want 1, got %d", len(d.Refs))
	}

	ref := d.Refs[0]
	if ref.RefModel != "Category" {
		t.Errorf("RefModel: want Category, got %q", ref.RefModel)
	}
	if ref.StoreMethod != "ListPostsByCategoryID" {
		t.Errorf("StoreMethod: want ListPostsByCategoryID, got %q", ref.StoreMethod)
	}
	if ref.ServiceMethod != "ListByCategoryID" {
		t.Errorf("ServiceMethod: want ListByCategoryID, got %q", ref.ServiceMethod)
	}
	if ref.ParamName != "categoryID" {
		t.Errorf("ParamName: want categoryID, got %q", ref.ParamName)
	}
}

// --------------------------------------------------------------------------
// NewData — aliased reference (manager:user:references)
// --------------------------------------------------------------------------

func TestNewData_AliasedUserRef(t *testing.T) {
	fields, err := ParseFields([]string{"manager:user:references"})
	if err != nil {
		t.Fatalf("ParseFields: %v", err)
	}
	d := NewData("Employee", fields, baseCfg())

	// aliased user ref is NOT treated as the primary user ref
	if d.HasUserRef {
		t.Error("HasUserRef should be false for aliased user ref (manager_id)")
	}
	if !d.HasNonUserRefs {
		t.Error("HasNonUserRefs should be true")
	}

	if len(d.Refs) != 1 {
		t.Fatalf("Refs length: want 1, got %d", len(d.Refs))
	}
	ref := d.Refs[0]

	if ref.RefModel != "User" {
		t.Errorf("RefModel: want User, got %q", ref.RefModel)
	}
	if ref.RefTable != "users" {
		t.Errorf("RefTable: want users, got %q", ref.RefTable)
	}
	// method names derived from alias, not ref model
	if ref.StoreMethod != "ListEmployeesByManagerID" {
		t.Errorf("StoreMethod: want ListEmployeesByManagerID, got %q", ref.StoreMethod)
	}
	if ref.ServiceMethod != "ListByManagerID" {
		t.Errorf("ServiceMethod: want ListByManagerID, got %q", ref.ServiceMethod)
	}
	if ref.ParamName != "managerID" {
		t.Errorf("ParamName: want managerID, got %q", ref.ParamName)
	}
	if ref.URLSegment != "manager" {
		t.Errorf("URLSegment: want manager, got %q", ref.URLSegment)
	}
	if ref.IsUserRef {
		t.Error("IsUserRef should be false for aliased ref")
	}
}

func TestNewData_TwoRefsToSameTable(t *testing.T) {
	// word_association: word:references + translate:word:references
	// both point to "words" but with different columns — no method collision
	fields, err := ParseFields([]string{"word:references", "translate:word:references"})
	if err != nil {
		t.Fatalf("ParseFields: %v", err)
	}
	d := NewData("WordAssociation", fields, baseCfg())

	if len(d.Refs) != 2 {
		t.Fatalf("Refs length: want 2, got %d", len(d.Refs))
	}

	r0, r1 := d.Refs[0], d.Refs[1]

	if r0.StoreMethod == r1.StoreMethod {
		t.Errorf("StoreMethod collision: both are %q", r0.StoreMethod)
	}
	if r0.ServiceMethod == r1.ServiceMethod {
		t.Errorf("ServiceMethod collision: both are %q", r0.ServiceMethod)
	}

	if r0.StoreMethod != "ListWordAssociationsByWordID" {
		t.Errorf("r0.StoreMethod: want ListWordAssociationsByWordID, got %q", r0.StoreMethod)
	}
	if r1.StoreMethod != "ListWordAssociationsByTranslateID" {
		t.Errorf("r1.StoreMethod: want ListWordAssociationsByTranslateID, got %q", r1.StoreMethod)
	}
}

// --------------------------------------------------------------------------
// SQL computed fields
// --------------------------------------------------------------------------

func TestNewData_SQLFragments_BasicFields(t *testing.T) {
	fields, err := ParseFields([]string{"title:string", "body:text"})
	if err != nil {
		t.Fatalf("ParseFields: %v", err)
	}
	d := NewData("Post", fields, baseCfg())

	// InsertCols should contain "title" and "body" but NOT "id"
	if !strings.Contains(d.InsertCols, "title") {
		t.Errorf("InsertCols missing 'title': %q", d.InsertCols)
	}
	if !strings.Contains(d.InsertCols, "body") {
		t.Errorf("InsertCols missing 'body': %q", d.InsertCols)
	}
	if strings.Contains(d.InsertCols, "id") {
		t.Errorf("InsertCols should not contain 'id': %q", d.InsertCols)
	}

	// SelectCols should contain "id", "title", "body", "created_at", "updated_at"
	for _, col := range []string{"id", "title", "body", "created_at", "updated_at"} {
		if !strings.Contains(d.SelectCols, col) {
			t.Errorf("SelectCols missing %q: %q", col, d.SelectCols)
		}
	}
}

func TestNewData_SQLFragments_WithRef(t *testing.T) {
	fields, err := ParseFields([]string{"title:string", "user:references"})
	if err != nil {
		t.Fatalf("ParseFields: %v", err)
	}
	d := NewData("Post", fields, baseCfg())

	// user_id should appear in InsertCols and SelectCols
	if !strings.Contains(d.InsertCols, "user_id") {
		t.Errorf("InsertCols missing 'user_id': %q", d.InsertCols)
	}
	if !strings.Contains(d.SelectCols, "user_id") {
		t.Errorf("SelectCols missing 'user_id': %q", d.SelectCols)
	}

	// UPDATE cols should NOT include user_id (refs are excluded from updates)
	if strings.Contains(d.SQLiteUpdate, "user_id") {
		t.Errorf("SQLiteUpdate should not contain 'user_id': %q", d.SQLiteUpdate)
	}
}

func TestNewData_HasTimeImport(t *testing.T) {
	fields, err := ParseFields([]string{"published:time"})
	if err != nil {
		t.Fatalf("ParseFields: %v", err)
	}
	d := NewData("Article", fields, baseCfg())
	if !d.HasTimeImport {
		t.Error("HasTimeImport should be true when a time field is present")
	}
}

func TestNewData_NeedsStrconv(t *testing.T) {
	for _, typ := range []string{"views:int", "price:float"} {
		t.Run(typ, func(t *testing.T) {
			fields, err := ParseFields([]string{typ})
			if err != nil {
				t.Fatalf("ParseFields: %v", err)
			}
			d := NewData("Item", fields, baseCfg())
			if !d.NeedsStrconv {
				t.Errorf("NeedsStrconv should be true for field %q", typ)
			}
		})
	}
}

// --------------------------------------------------------------------------
// Helpers: ToCamel, Pluralize, ToSnake
// --------------------------------------------------------------------------

func TestToCamel(t *testing.T) {
	cases := []struct{ in, want string }{
		{"user_id", "UserID"},
		{"title", "Title"},
		{"created_at", "CreatedAt"},
		{"uuid", "UUID"},
		{"url", "URL"},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			got := ToCamel(tc.in)
			if got != tc.want {
				t.Errorf("ToCamel(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestPluralize(t *testing.T) {
	cases := []struct{ in, want string }{
		{"post", "posts"},
		{"category", "categories"},
		{"box", "boxes"},
		{"user", "users"},
		// "status" ends in "s" (not "ss"), so Pluralize returns it unchanged
		{"status", "status"},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			got := Pluralize(tc.in)
			if got != tc.want {
				t.Errorf("Pluralize(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestToSnake(t *testing.T) {
	cases := []struct{ in, want string }{
		{"Post", "post"},
		{"BlogPost", "blog_post"},
		{"UserID", "user_i_d"},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			got := ToSnake(tc.in)
			if got != tc.want {
				t.Errorf("ToSnake(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
