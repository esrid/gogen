package render

import (
	"strings"
	"testing"

	"github.com/esrid/gogen/internal/config"
	"github.com/esrid/gogen/internal/scaffold"
)

// newData is a helper that builds scaffold.Data from field strings.
func newData(modelName string, fields []string, db, renderMode string, auth, protected bool) *scaffold.Data {
	parsed, err := scaffold.ParseFields(fields)
	if err != nil {
		panic("newData: ParseFields: " + err.Error())
	}
	cfg := &config.ProjectConfig{
		ProjectName: "testapp",
		ModulePath:  "github.com/example/testapp",
		DB:          db,
		RenderMode:  renderMode,
		Auth:        auth,
	}
	d := scaffold.NewData(modelName, parsed, cfg)
	d.Protected = protected
	return d
}

// newProjectConfig builds a minimal ProjectConfig for project-level templates.
func newProjectConfig(db, renderMode string, auth bool) *config.ProjectConfig {
	return &config.ProjectConfig{
		ProjectName: "testapp",
		ModulePath:  "github.com/example/testapp",
		DB:          db,
		RenderMode:  renderMode,
		Auth:        auth,
		Year:        2024,
	}
}

// mustFile renders the template and fails the test if there is an error.
func mustFile(t *testing.T, tmplPath string, data any) string {
	t.Helper()
	out, err := File(tmplPath, data)
	if err != nil {
		t.Fatalf("File(%q): %v", tmplPath, err)
	}
	return string(out)
}

// assertContains fails the test if the output does not contain the expected substring.
func assertContains(t *testing.T, out, want string) {
	t.Helper()
	if !strings.Contains(out, want) {
		t.Errorf("expected output to contain %q\ngot:\n%s", want, out)
	}
}

// assertNotContains fails the test if the output contains the unexpected substring.
func assertNotContains(t *testing.T, out, unwanted string) {
	t.Helper()
	if strings.Contains(out, unwanted) {
		t.Errorf("expected output NOT to contain %q\ngot:\n%s", unwanted, out)
	}
}

// --------------------------------------------------------------------------
// scaffold/domain.go.tmpl
// --------------------------------------------------------------------------

func TestFile_DomainTemplate(t *testing.T) {
	d := newData("Post", []string{"title:string", "body:text"}, "sqlite", "api", false, false)
	out := mustFile(t, "scaffold/domain.go.tmpl", d)

	assertContains(t, out, "type Post struct")
	// go/format aligns struct fields with tabs, so check field names without full type suffix
	assertContains(t, out, "Title")
	assertContains(t, out, "Body")
	assertContains(t, out, "ID")
	assertContains(t, out, "string")
	assertContains(t, out, "CreatedAt")
	assertContains(t, out, "UpdatedAt")
}

// --------------------------------------------------------------------------
// scaffold/handler.go.tmpl — api mode
// --------------------------------------------------------------------------

func TestFile_HandlerTemplate_APIMode(t *testing.T) {
	d := newData("Post", []string{"title:string", "body:text"}, "sqlite", "api", false, false)
	out := mustFile(t, "scaffold/handler.go.tmpl", d)

	assertContains(t, out, "type PostHandler struct")
	assertContains(t, out, "func (h *PostHandler) Register")
	assertContains(t, out, `r.Route("/posts"`)
	assertContains(t, out, "func NewPostHandler")
	assertNotContains(t, out, "PostAPI")
	assertNotContains(t, out, "/api/posts")
}

// --------------------------------------------------------------------------
// scaffold/handler.go.tmpl — both mode
// --------------------------------------------------------------------------

func TestFile_HandlerTemplate_BothMode(t *testing.T) {
	d := newData("Post", []string{"title:string"}, "sqlite", "both", false, false)
	out := mustFile(t, "scaffold/handler.go.tmpl", d)

	assertContains(t, out, "type PostAPIHandler struct")
	assertContains(t, out, `r.Route("/api/posts"`)
	assertContains(t, out, "func NewPostAPIHandler")
	assertNotContains(t, out, "type PostHandler struct")
}

// --------------------------------------------------------------------------
// scaffold/handler_ssr.go.tmpl
// --------------------------------------------------------------------------

func TestFile_HandlerSSRTemplate(t *testing.T) {
	d := newData("Post", []string{"title:string"}, "sqlite", "ssr", false, false)
	out := mustFile(t, "scaffold/handler_ssr.go.tmpl", d)

	assertContains(t, out, "type PostHandler struct")
	assertContains(t, out, "package web")
	assertContains(t, out, `r.Route("/posts"`)
	assertContains(t, out, "func NewPostHandler")
}

// --------------------------------------------------------------------------
// scaffold/port.go.tmpl — with category:references (non-user ref)
// --------------------------------------------------------------------------

func TestFile_PortTemplate_WithNonUserRef(t *testing.T) {
	d := newData("Post", []string{"title:string", "category:references"}, "sqlite", "api", false, false)
	out := mustFile(t, "scaffold/port.go.tmpl", d)

	assertContains(t, out, "ListPostsByCategoryID")
	assertContains(t, out, "ListByCategoryID")
	assertContains(t, out, "type PostStore interface")
	assertContains(t, out, "type PostService interface")
}

// --------------------------------------------------------------------------
// scaffold/port.go.tmpl — user ref should not produce public list method
// --------------------------------------------------------------------------

func TestFile_PortTemplate_UserRefAppearsInInterface(t *testing.T) {
	d := newData("Post", []string{"user:references"}, "sqlite", "api", false, false)
	out := mustFile(t, "scaffold/port.go.tmpl", d)

	// The port template ranges over ALL refs including user refs
	assertContains(t, out, "ListPostsByUserID")
	assertContains(t, out, "ListByUserID")
}

// --------------------------------------------------------------------------
// new/base/bootstrap/wire_gen.go.tmpl — no auth
// --------------------------------------------------------------------------

func TestFile_WireGenTemplate_NoAuth(t *testing.T) {
	cfg := newProjectConfig("sqlite", "api", false)
	out := mustFile(t, "new/base/bootstrap/wire_gen.go.tmpl", cfg)

	assertContains(t, out, "PublicControllers")
	assertContains(t, out, "ProtectedControllers")
	assertNotContains(t, out, "AuthHandler")
	assertNotContains(t, out, "SessionService")
}

// --------------------------------------------------------------------------
// new/base/bootstrap/wire_gen.go.tmpl — with auth
// --------------------------------------------------------------------------

func TestFile_WireGenTemplate_WithAuth(t *testing.T) {
	cfg := newProjectConfig("sqlite", "api", true)
	out := mustFile(t, "new/base/bootstrap/wire_gen.go.tmpl", cfg)

	assertContains(t, out, "*api.AuthHandler")
	assertContains(t, out, "SessionService")
	assertContains(t, out, "h.PublicControllers = append(h.PublicControllers, h.Auth)")
}
