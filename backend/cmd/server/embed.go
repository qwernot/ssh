package main

import (
	"embed"
	"io/fs"
	"strings"

	"github.com/gin-gonic/gin"
)

//go:embed web/*
var embeddedFiles embed.FS

// ServeEmbeddedFrontend serves the embedded frontend files with SPA fallback.
func ServeEmbeddedFrontend(r *gin.Engine) {
	subFS, err := fs.Sub(embeddedFiles, "web")
	if err != nil {
		serveFallback(r)
		return
	}

	// Check if index.html exists
	if _, err := subFS.Open("index.html"); err != nil {
		serveFallback(r)
		return
	}

	// Read index.html content for SPA fallback
	indexHTML, _ := fs.ReadFile(subFS, "index.html")

	r.NoRoute(func(c *gin.Context) {
		path := c.Request.URL.Path

		// Skip API routes and health check
		if strings.HasPrefix(path, "/api/") || path == "/health" {
			c.JSON(404, gin.H{"error": "API not found"})
			return
		}

		// Try to serve the requested file
		cleanPath := strings.TrimPrefix(path, "/")
		if cleanPath == "" {
			cleanPath = "index.html"
		}

		// Try to read the file from embedded FS
		data, err := fs.ReadFile(subFS, cleanPath)
		if err == nil {
			// Determine content type
			contentType := guessContentType(cleanPath)
			c.Data(200, contentType, data)
			return
		}

		// SPA fallback: serve index.html for client-side routing
		c.Data(200, "text/html; charset=utf-8", indexHTML)
	})
}

func guessContentType(path string) string {
	switch {
	case strings.HasSuffix(path, ".html"):
		return "text/html; charset=utf-8"
	case strings.HasSuffix(path, ".css"):
		return "text/css; charset=utf-8"
	case strings.HasSuffix(path, ".js"):
		return "application/javascript; charset=utf-8"
	case strings.HasSuffix(path, ".json"):
		return "application/json"
	case strings.HasSuffix(path, ".svg"):
		return "image/svg+xml"
	case strings.HasSuffix(path, ".png"):
		return "image/png"
	case strings.HasSuffix(path, ".ico"):
		return "image/x-icon"
	case strings.HasSuffix(path, ".woff2"):
		return "font/woff2"
	case strings.HasSuffix(path, ".woff"):
		return "font/woff"
	default:
		return "application/octet-stream"
	}
}

func serveFallback(r *gin.Engine) {
	r.NoRoute(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/api/") {
			c.JSON(404, gin.H{"error": "API not found"})
			return
		}
		c.Data(200, "text/html; charset=utf-8", []byte(`<!DOCTYPE html><html><head><meta charset="utf-8"><title>Shelly</title></head><body><h2>Shelly SSH Manager</h2><p>Frontend not embedded. Copy frontend/dist/* to backend/cmd/server/web/ then rebuild.</p></body></html>`))
	})
}
