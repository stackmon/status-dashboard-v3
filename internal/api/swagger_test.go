package api

import (
	"crypto/tls"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() { //nolint:gochecknoinits // set gin test mode before tests
	gin.SetMode(gin.TestMode)
}

func requireMap(t *testing.T, m map[string]interface{}, key string) map[string]interface{} {
	t.Helper()
	v, ok := m[key]
	require.True(t, ok, "key %q not found", key)
	result, ok := v.(map[string]interface{})
	require.True(t, ok, "key %q is not map[string]interface{}", key)
	return result
}

func requireSlice(t *testing.T, m map[string]interface{}, key string) []interface{} {
	t.Helper()
	v, ok := m[key]
	require.True(t, ok, "key %q not found", key)
	result, ok := v.([]interface{})
	require.True(t, ok, "key %q is not []interface{}", key)
	return result
}

func requireServerURL(t *testing.T, spec map[string]interface{}) string {
	t.Helper()
	servers := requireSlice(t, spec, "servers")
	require.NotEmpty(t, servers)
	srv, ok := servers[0].(map[string]interface{})
	require.True(t, ok)
	url, ok := srv["url"].(string)
	require.True(t, ok)
	return url
}

func withTempOpenAPIFile(t *testing.T, content string, fn func()) {
	t.Helper()
	origDir, err := os.Getwd()
	require.NoError(t, err)

	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, openAPISpecPath), []byte(content), 0o600))
	require.NoError(t, os.Chdir(tmpDir))
	defer func() { require.NoError(t, os.Chdir(origDir)) }()

	fn()
}

func withNoOpenAPIFile(t *testing.T, fn func()) {
	t.Helper()
	origDir, err := os.Getwd()
	require.NoError(t, err)

	tmpDir := t.TempDir()
	require.NoError(t, os.Chdir(tmpDir))
	defer func() { require.NoError(t, os.Chdir(origDir)) }()

	fn()
}

func TestFilteredOpenAPIHandler_FileNotFound(t *testing.T) {
	withNoOpenAPIFile(t, func() {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/openapi", nil)

		filteredOpenAPIHandler()(c)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		assert.Contains(t, w.Body.String(), "failed to read OpenAPI spec")
	})
}

func TestFilteredOpenAPIHandler_InvalidYAML(t *testing.T) {
	withTempOpenAPIFile(t, ":\ninvalid: [yaml: {{{\n", func() {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/openapi", nil)

		filteredOpenAPIHandler()(c)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		assert.Contains(t, w.Body.String(), "failed to parse OpenAPI spec")
	})
}

func TestFilteredOpenAPIHandler_ValidSpec(t *testing.T) {
	specYAML := `
info:
  title: Test API
  description: "Some text\n## Authentication\nSecret stuff"
paths:
  /status:
    get:
      tags:
        - Status
      summary: Get status
    post:
      summary: Create status
  /auth/login:
    post:
      summary: Login
tags:
  - name: Status
  - name: Auth
security:
  - bearerAuth: []
components:
  securitySchemes:
    bearerAuth:
      type: http
  schemas:
    TokenResponse:
      type: object
    StatusModel:
      type: object
      properties:
        name:
          type: string
        creator:
          type: string
      required:
        - name
        - creator
`
	withTempOpenAPIFile(t, specYAML, func() {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/openapi", nil)
		c.Request.Host = "example.com"

		filteredOpenAPIHandler()(c)

		require.Equal(t, http.StatusOK, w.Code)

		var result map[string]interface{}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))

		paths := requireMap(t, result, "paths")
		assert.Contains(t, paths, "/status")
		assert.NotContains(t, paths, "/auth/login")

		statusMethods := requireMap(t, paths, "/status")
		assert.Contains(t, statusMethods, "get")
		assert.NotContains(t, statusMethods, "post")

		assert.NotContains(t, result, "security")
		assert.Equal(t, "http://example.com", requireServerURL(t, result))

		tags := requireSlice(t, result, "tags")
		assert.Len(t, tags, 1)

		components := requireMap(t, result, "components")
		schemas := requireMap(t, components, "schemas")
		assert.NotContains(t, schemas, "TokenResponse")
		assert.Contains(t, schemas, "StatusModel")

		statusSchema := requireMap(t, schemas, "StatusModel")
		props := requireMap(t, statusSchema, "properties")
		assert.NotContains(t, props, "creator")
		assert.Contains(t, props, "name")
	})
}

func TestStripNonGetMethods_PathsNotMap(t *testing.T) {
	spec := map[string]interface{}{"paths": "not-a-map"}
	stripNonGetMethods(spec)
	assert.Equal(t, "not-a-map", spec["paths"])
}

func TestStripNonGetMethods_NormalPaths(t *testing.T) {
	spec := map[string]interface{}{
		"paths": map[string]interface{}{
			"/items": map[string]interface{}{
				"get":  map[string]interface{}{"tags": []interface{}{"Items"}},
				"post": map[string]interface{}{},
			},
		},
		"tags": []interface{}{
			map[string]interface{}{"name": "Items"},
			map[string]interface{}{"name": "Unused"},
		},
	}
	stripNonGetMethods(spec)

	paths := requireMap(t, spec, "paths")
	items := requireMap(t, paths, "/items")
	assert.Contains(t, items, "get")
	assert.NotContains(t, items, "post")

	tags := requireSlice(t, spec, "tags")
	assert.Len(t, tags, 1)
}

func TestFilterGETPaths(t *testing.T) {
	paths := map[string]interface{}{
		"/auth/token":  map[string]interface{}{"post": "login"},
		"/bad":         "not-a-map",
		"/no-get":      map[string]interface{}{"post": "create"},
		"/has-get":     map[string]interface{}{"get": map[string]interface{}{"tags": []interface{}{"Public"}}},
		"/get-no-tags": map[string]interface{}{"get": map[string]interface{}{}},
	}

	filtered, usedTags := filterGETPaths(paths)

	assert.NotContains(t, filtered, "/auth/token")
	assert.NotContains(t, filtered, "/bad")
	assert.NotContains(t, filtered, "/no-get")
	assert.Contains(t, filtered, "/has-get")
	assert.Contains(t, filtered, "/get-no-tags")
	assert.Contains(t, usedTags, "Public")
}

func TestCollectTags(t *testing.T) {
	t.Run("not a map", func(t *testing.T) {
		tags := make(map[string]struct{})
		collectTags("string-value", tags)
		assert.Empty(t, tags)
	})

	t.Run("no tags key", func(t *testing.T) {
		tags := make(map[string]struct{})
		collectTags(map[string]interface{}{"summary": "x"}, tags)
		assert.Empty(t, tags)
	})

	t.Run("tags with non-string items", func(t *testing.T) {
		tags := make(map[string]struct{})
		collectTags(map[string]interface{}{
			"tags": []interface{}{42, "Valid"},
		}, tags)
		assert.Contains(t, tags, "Valid")
		assert.Len(t, tags, 1)
	})
}

func TestPruneUnusedTags(t *testing.T) {
	t.Run("tags not a slice", func(t *testing.T) {
		spec := map[string]interface{}{"tags": "not-slice"}
		pruneUnusedTags(spec, map[string]struct{}{"X": {}})
		assert.Equal(t, "not-slice", spec["tags"])
	})

	t.Run("tag with empty name skipped", func(t *testing.T) {
		spec := map[string]interface{}{
			"tags": []interface{}{
				"not-a-map",
				map[string]interface{}{"name": 123},
				map[string]interface{}{"name": "Used"},
			},
		}
		pruneUnusedTags(spec, map[string]struct{}{"Used": {}})
		kept := requireSlice(t, spec, "tags")
		assert.Len(t, kept, 1)
	})

	t.Run("unused tag removed", func(t *testing.T) {
		spec := map[string]interface{}{
			"tags": []interface{}{
				map[string]interface{}{"name": "Keep"},
				map[string]interface{}{"name": "Drop"},
			},
		}
		pruneUnusedTags(spec, map[string]struct{}{"Keep": {}})
		kept := requireSlice(t, spec, "tags")
		assert.Len(t, kept, 1)
	})
}

func TestExtractTagName(t *testing.T) {
	tests := []struct {
		name string
		tag  interface{}
		want string
	}{
		{"not a map", "string", ""},
		{"name not string", map[string]interface{}{"name": 42}, ""},
		{"valid name", map[string]interface{}{"name": "MyTag"}, "MyTag"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, extractTagName(tt.tag))
		})
	}
}

func TestOverrideServerURL(t *testing.T) {
	t.Run("plain HTTP", func(t *testing.T) {
		spec := map[string]interface{}{}
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.Host = "api.example.com"
		overrideServerURL(spec, r)
		assert.Equal(t, "http://api.example.com", requireServerURL(t, spec))
	})

	t.Run("TLS request", func(t *testing.T) {
		spec := map[string]interface{}{}
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.Host = "secure.example.com"
		r.TLS = &tls.ConnectionState{}
		overrideServerURL(spec, r)
		assert.Equal(t, "https://secure.example.com", requireServerURL(t, spec))
	})

	t.Run("X-Forwarded-Proto https", func(t *testing.T) {
		spec := map[string]interface{}{}
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.Host = "proxy.example.com"
		r.Header.Set("X-Forwarded-Proto", "https")
		overrideServerURL(spec, r)
		assert.Equal(t, "https://proxy.example.com", requireServerURL(t, spec))
	})
}

func TestStripAuthDetails(t *testing.T) {
	spec := map[string]interface{}{
		"security": []interface{}{map[string]interface{}{"bearerAuth": []interface{}{}}},
		"info":     map[string]interface{}{"description": "Hello\n## Authentication\nSecret"},
		"components": map[string]interface{}{
			"securitySchemes": map[string]interface{}{"bearerAuth": "x"},
			"schemas":         map[string]interface{}{},
		},
	}
	stripAuthDetails(spec)

	assert.NotContains(t, spec, "security")
	info := requireMap(t, spec, "info")
	assert.Equal(t, "Hello", info["description"])
}

func TestStripInfoDescription(t *testing.T) {
	t.Run("info not a map", func(t *testing.T) {
		spec := map[string]interface{}{"info": "string"}
		stripInfoDescription(spec)
		assert.Equal(t, "string", spec["info"])
	})

	t.Run("description not a string", func(t *testing.T) {
		spec := map[string]interface{}{"info": map[string]interface{}{"description": 42}}
		stripInfoDescription(spec)
		info := requireMap(t, spec, "info")
		assert.Equal(t, 42, info["description"])
	})

	t.Run("no authentication section", func(t *testing.T) {
		spec := map[string]interface{}{"info": map[string]interface{}{"description": "Just a description"}}
		stripInfoDescription(spec)
		info := requireMap(t, spec, "info")
		assert.Equal(t, "Just a description", info["description"])
	})

	t.Run("with authentication section", func(t *testing.T) {
		spec := map[string]interface{}{
			"info": map[string]interface{}{"description": "Intro\n## Authentication\nDetails"},
		}
		stripInfoDescription(spec)
		info := requireMap(t, spec, "info")
		assert.Equal(t, "Intro", info["description"])
	})
}

func TestStripAuthSchemas(t *testing.T) {
	t.Run("components not a map", func(t *testing.T) {
		spec := map[string]interface{}{"components": "string"}
		stripAuthSchemas(spec)
		assert.Equal(t, "string", spec["components"])
	})

	t.Run("schemas not a map", func(t *testing.T) {
		spec := map[string]interface{}{
			"components": map[string]interface{}{
				"securitySchemes": "x",
				"schemas":         "not-a-map",
			},
		}
		stripAuthSchemas(spec)
		components := requireMap(t, spec, "components")
		assert.NotContains(t, components, "securitySchemes")
	})

	t.Run("Token schemas deleted and RBAC fields stripped", func(t *testing.T) {
		spec := map[string]interface{}{
			"components": map[string]interface{}{
				"schemas": map[string]interface{}{
					"TokenRefresh": map[string]interface{}{"type": "object"},
					"Status": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"name":    map[string]interface{}{"type": "string"},
							"creator": map[string]interface{}{"type": "string"},
						},
					},
				},
			},
		}
		stripAuthSchemas(spec)

		components := requireMap(t, spec, "components")
		schemas := requireMap(t, components, "schemas")
		assert.NotContains(t, schemas, "TokenRefresh")
		assert.Contains(t, schemas, "Status")

		status := requireMap(t, schemas, "Status")
		props := requireMap(t, status, "properties")
		assert.NotContains(t, props, "creator")
	})
}

func TestStripRBACFields(t *testing.T) {
	t.Run("not a map", func(_ *testing.T) {
		stripRBACFields("not-a-map")
	})

	t.Run("no properties or required", func(t *testing.T) {
		schema := map[string]interface{}{"type": "object"}
		stripRBACFields(schema)
		assert.Equal(t, "object", schema["type"])
	})

	t.Run("properties with rbac fields", func(t *testing.T) {
		schema := map[string]interface{}{
			"properties": map[string]interface{}{
				"name":          map[string]interface{}{"type": "string"},
				"creator":       map[string]interface{}{"type": "string"},
				"contact_email": map[string]interface{}{"type": "string"},
				"version":       map[string]interface{}{"type": "string"},
			},
		}
		stripRBACFields(schema)

		props := requireMap(t, schema, "properties")
		assert.Contains(t, props, "name")
		assert.NotContains(t, props, "creator")
		assert.NotContains(t, props, "contact_email")
		assert.NotContains(t, props, "version")
	})

	t.Run("required with rbac and non-string items", func(t *testing.T) {
		schema := map[string]interface{}{
			"required": []interface{}{"name", "creator", 42, "contact_email", "version"},
		}
		stripRBACFields(schema)

		required := requireSlice(t, schema, "required")
		assert.Len(t, required, 2)
		assert.Contains(t, required, "name")
		assert.Contains(t, required, 42)
	})
}
