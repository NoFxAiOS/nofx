package api

import (
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
)

// RouteDoc holds documentation for a single API route.
type RouteDoc struct {
	Method      string
	Path        string
	Description string
	Schema      string // optional: full parameter/body schema documentation
}

// routeRegistry stores all documented routes. Populated via s.route() calls in setupRoutes.
var routeRegistry []RouteDoc

// route registers an HTTP route with a one-line description.
func (s *Server) route(g *gin.RouterGroup, method, path, description string, h gin.HandlerFunc) {
	s.routeWithSchema(g, method, path, description, "", h)
}

// routeWithSchema registers an HTTP route with full parameter schema documentation.
// schema is injected verbatim into the API docs seen by the LLM.
func (s *Server) routeWithSchema(g *gin.RouterGroup, method, path, description, schema string, h gin.HandlerFunc) {
	fullPath := strings.TrimSuffix(g.BasePath(), "/") + "/" + strings.TrimPrefix(path, "/")
	routeRegistry = append(routeRegistry, RouteDoc{
		Method:      method,
		Path:        fullPath,
		Description: description,
		Schema:      schema,
	})
	switch method {
	case "GET":
		g.GET(path, h)
	case "POST":
		g.POST(path, h)
	case "PUT":
		g.PUT(path, h)
	case "DELETE":
		g.DELETE(path, h)
	}
}

// GetAPIDocs returns formatted API documentation for injection into the LLM system prompt.
// Routes with schema documentation include full parameter details.
func GetAPIDocs() string {
	var sb strings.Builder
	for _, r := range routeRegistry {
		sb.WriteString(fmt.Sprintf("%-8s %s\n", r.Method, r.Path))
		sb.WriteString(fmt.Sprintf("         %s\n", r.Description))
		if r.Schema != "" {
			// Indent each schema line for readability
			for _, line := range strings.Split(strings.TrimSpace(r.Schema), "\n") {
				sb.WriteString("         ")
				sb.WriteString(line)
				sb.WriteByte('\n')
			}
		}
		sb.WriteByte('\n')
	}
	return sb.String()
}
