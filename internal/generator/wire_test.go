package generator

import (
	"strings"
	"testing"

	"github.com/esrid/gogen/internal/config"
)

// cfg is a helper that builds a GogenYAML for testing.
func cfg(module, db, renderMode string, auth bool, scaffolds map[string]*config.ScaffoldMeta) *config.GogenYAML {
	return &config.GogenYAML{
		Module:     module,
		DB:         db,
		RenderMode: renderMode,
		Auth:       auth,
		Scaffolds:  scaffolds,
	}
}

// noScaffolds is a convenience nil map.
var noScaffolds map[string]*config.ScaffoldMeta

func assertContains(t *testing.T, out, want string) {
	t.Helper()
	if !strings.Contains(out, want) {
		t.Errorf("expected output to contain %q\ngot:\n%s", want, out)
	}
}

func assertNotContains(t *testing.T, out, unwanted string) {
	t.Helper()
	if strings.Contains(out, unwanted) {
		t.Errorf("expected output NOT to contain %q\ngot:\n%s", unwanted, out)
	}
}

// --------------------------------------------------------------------------
// No auth, no scaffolds
// --------------------------------------------------------------------------

func TestWireGenContent_NoAuth_NoScaffolds(t *testing.T) {
	c := cfg("github.com/example/myapp", "sqlite", "api", false, noScaffolds)
	out := string(WireGenContent(c))

	assertContains(t, out, "PublicControllers    []api.Controller")
	assertContains(t, out, "ProtectedControllers []api.Controller")
	assertNotContains(t, out, "AuthHandler")
	assertNotContains(t, out, "application")
	assertNotContains(t, out, "domain.SessionService")
}

// --------------------------------------------------------------------------
// Auth, no scaffolds
// --------------------------------------------------------------------------

func TestWireGenContent_Auth_NoScaffolds(t *testing.T) {
	c := cfg("github.com/example/myapp", "sqlite", "api", true, noScaffolds)
	out := string(WireGenContent(c))

	assertContains(t, out, "*api.AuthHandler")
	assertContains(t, out, "domain.SessionService")
	assertContains(t, out, "h.PublicControllers = append(h.PublicControllers, h.Auth)")
	// Auth implies application and domain imports
	assertContains(t, out, "internal/application")
	assertContains(t, out, "internal/domain")
}

// --------------------------------------------------------------------------
// No auth, one scaffold "Post" in api mode
// --------------------------------------------------------------------------

func TestWireGenContent_NoAuth_OneScaffold_API(t *testing.T) {
	scaffolds := map[string]*config.ScaffoldMeta{
		"Post": {Protected: false},
	}
	c := cfg("github.com/example/myapp", "sqlite", "api", false, scaffolds)
	out := string(WireGenContent(c))

	assertContains(t, out, "Post *api.PostHandler")
	assertContains(t, out, "postSvc := application.NewPostService(dbStore)")
	assertContains(t, out, "h.Post = api.NewPostHandler(postSvc, logger)")
	assertContains(t, out, "h.PublicControllers = append(h.PublicControllers, h.Post)")
	assertNotContains(t, out, "ProtectedControllers = append(h.ProtectedControllers, h.Post)")
	assertNotContains(t, out, "AuthHandler")
}

// --------------------------------------------------------------------------
// Auth, one scaffold "Post", protected=true
// --------------------------------------------------------------------------

func TestWireGenContent_Auth_OneScaffold_Protected(t *testing.T) {
	scaffolds := map[string]*config.ScaffoldMeta{
		"Post": {Protected: true},
	}
	c := cfg("github.com/example/myapp", "sqlite", "api", true, scaffolds)
	out := string(WireGenContent(c))

	assertContains(t, out, "h.ProtectedControllers = append(h.ProtectedControllers, h.Post)")
	assertNotContains(t, out, "h.PublicControllers = append(h.PublicControllers, h.Post)")
}

// --------------------------------------------------------------------------
// Auth, one scaffold "Post" in "both" mode
// --------------------------------------------------------------------------

func TestWireGenContent_Auth_OneScaffold_BothMode(t *testing.T) {
	scaffolds := map[string]*config.ScaffoldMeta{
		"Post": {Protected: false},
	}
	c := cfg("github.com/example/myapp", "sqlite", "both", true, scaffolds)
	out := string(WireGenContent(c))

	// In "both" mode with SSR, the main handler is a web handler
	assertContains(t, out, "Post *web.PostHandler")
	// "both" mode also adds an API handler
	assertContains(t, out, "PostAPI *api.PostAPIHandler")
	assertContains(t, out, "h.PostAPI = api.NewPostAPIHandler(postSvc, logger)")
}

// --------------------------------------------------------------------------
// Multiple scaffolds appear in sorted (alphabetical) order
// --------------------------------------------------------------------------

func TestWireGenContent_MultipleScaffolds_SortedOrder(t *testing.T) {
	scaffolds := map[string]*config.ScaffoldMeta{
		"Zebra":  {Protected: false},
		"Alpha":  {Protected: false},
		"Medium": {Protected: false},
	}
	c := cfg("github.com/example/myapp", "sqlite", "api", false, scaffolds)
	out := string(WireGenContent(c))

	// All three handlers must appear
	assertContains(t, out, "Alpha *api.AlphaHandler")
	assertContains(t, out, "Medium *api.MediumHandler")
	assertContains(t, out, "Zebra *api.ZebraHandler")

	// Verify alphabetical order: Alpha < Medium < Zebra
	idxAlpha := strings.Index(out, "Alpha *api.AlphaHandler")
	idxMedium := strings.Index(out, "Medium *api.MediumHandler")
	idxZebra := strings.Index(out, "Zebra *api.ZebraHandler")

	if idxAlpha < 0 || idxMedium < 0 || idxZebra < 0 {
		t.Fatal("one or more handler fields not found in output")
	}
	if !(idxAlpha < idxMedium && idxMedium < idxZebra) {
		t.Errorf("handlers not in alphabetical order: Alpha@%d Medium@%d Zebra@%d",
			idxAlpha, idxMedium, idxZebra)
	}
}

// --------------------------------------------------------------------------
// SSR-only mode: scaffold uses web package, not api
// --------------------------------------------------------------------------

func TestWireGenContent_NoAuth_OneScaffold_SSRMode(t *testing.T) {
	scaffolds := map[string]*config.ScaffoldMeta{
		"Post": {Protected: false},
	}
	c := cfg("github.com/example/myapp", "sqlite", "ssr", false, scaffolds)
	out := string(WireGenContent(c))

	assertContains(t, out, "Post *web.PostHandler")
	assertContains(t, out, "h.Post = web.NewPostHandler(postSvc, logger)")
	assertNotContains(t, out, "*api.PostHandler")
	// web import should be present
	assertContains(t, out, "internal/adapters/web")
}

// --------------------------------------------------------------------------
// Package declaration is always "bootstrap"
// --------------------------------------------------------------------------

func TestWireGenContent_PackageDeclaration(t *testing.T) {
	c := cfg("github.com/example/myapp", "sqlite", "api", false, noScaffolds)
	out := string(WireGenContent(c))

	if !strings.HasPrefix(out, "package bootstrap") {
		t.Errorf("expected output to start with 'package bootstrap', got prefix: %q",
			out[:min(len(out), 40)])
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
