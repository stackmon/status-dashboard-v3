package api

import (
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"gopkg.in/yaml.v3"
)

const openAPISpecPath = "openapi.yaml"

// filteredOpenAPIHandler serves the OpenAPI spec with only GET methods.
func filteredOpenAPIHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		data, err := os.ReadFile(openAPISpecPath)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read OpenAPI spec"})
			return
		}

		var spec map[string]interface{}
		if unmarshalErr := yaml.Unmarshal(data, &spec); unmarshalErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to parse OpenAPI spec"})
			return
		}

		stripNonGetMethods(spec)
		overrideServerURL(spec, c.Request)

		c.JSON(http.StatusOK, spec)
	}
}

// stripNonGetMethods removes all non-GET operations from the OpenAPI spec
// and prunes unused tags.
func stripNonGetMethods(spec map[string]interface{}) {
	paths, ok := spec["paths"].(map[string]interface{})
	if !ok {
		return
	}

	filtered, usedTags := filterGETPaths(paths)
	spec["paths"] = filtered

	pruneUnusedTags(spec, usedTags)
}

func filterGETPaths(paths map[string]interface{}) (map[string]interface{}, map[string]struct{}) {
	filtered := make(map[string]interface{})
	usedTags := make(map[string]struct{})

	for path, methods := range paths {
		if strings.HasPrefix(path, "/auth/") {
			continue
		}

		methodMap, isMap := methods.(map[string]interface{})
		if !isMap {
			continue
		}

		getOp, hasGet := methodMap["get"]
		if !hasGet {
			continue
		}

		filtered[path] = map[string]interface{}{"get": getOp}
		collectTags(getOp, usedTags)
	}

	return filtered, usedTags
}

func collectTags(operation interface{}, tags map[string]struct{}) {
	opMap, isMap := operation.(map[string]interface{})
	if !isMap {
		return
	}

	tagList, hasTags := opMap["tags"].([]interface{})
	if !hasTags {
		return
	}

	for _, t := range tagList {
		if tag, isString := t.(string); isString {
			tags[tag] = struct{}{}
		}
	}
}

func pruneUnusedTags(spec map[string]interface{}, usedTags map[string]struct{}) {
	tags, hasTags := spec["tags"].([]interface{})
	if !hasTags {
		return
	}

	var kept []interface{}

	for _, t := range tags {
		if name := extractTagName(t); name != "" {
			if _, used := usedTags[name]; used {
				kept = append(kept, t)
			}
		}
	}

	spec["tags"] = kept
}

func extractTagName(tag interface{}) string {
	tagMap, isMap := tag.(map[string]interface{})
	if !isMap {
		return ""
	}

	name, isString := tagMap["name"].(string)
	if !isString {
		return ""
	}

	return name
}

// overrideServerURL replaces the servers block with the actual request host.
func overrideServerURL(spec map[string]interface{}, r *http.Request) {
	scheme := "http"
	if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}

	spec["servers"] = []interface{}{
		map[string]interface{}{
			"url": fmt.Sprintf("%s://%s", scheme, r.Host),
		},
	}
}
