package web

import "strings"

// isAssetPath returns true if the path looks like a static asset.
func isAssetPath(path string) bool {
	exts := []string{".js", ".css", ".png", ".jpg", ".jpeg", ".svg", ".ico", ".woff", ".woff2", ".ttf", ".map", ".json"}
	for _, ext := range exts {
		if strings.HasSuffix(path, ext) {
			return true
		}
	}
	return false
}

// isVersionedAsset returns true if the path contains a hash (versioned asset).
// Vite generates files like: app-abc123.js, style-def456.css
// These files have content hashes and can be cached forever.
func isVersionedAsset(path string) bool {
	// Check if filename contains a hash pattern: name-[hash].ext
	// Hash is typically 8+ alphanumeric characters
	parts := strings.Split(path, "/")
	filename := parts[len(parts)-1]

	// Remove extension
	dotIdx := strings.LastIndex(filename, ".")
	if dotIdx == -1 {
		return false
	}
	nameWithoutExt := filename[:dotIdx]

	// Check if name contains a dash followed by hash-like string
	dashIdx := strings.LastIndex(nameWithoutExt, "-")
	if dashIdx == -1 {
		return false
	}

	potentialHash := nameWithoutExt[dashIdx+1:]
	// Hash should be at least 8 characters and alphanumeric
	if len(potentialHash) < 8 {
		return false
	}

	for _, c := range potentialHash {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')) {
			return false
		}
	}

	return true
}

// isImageFile returns true if the path is an image file.
func isImageFile(path string) bool {
	imageExts := []string{".png", ".jpg", ".jpeg", ".gif", ".webp", ".svg", ".ico", ".bmp"}
	for _, ext := range imageExts {
		if strings.HasSuffix(path, ext) {
			return true
		}
	}
	return false
}

// getContentType returns the content type for a file path.
func getContentType(path string) string {
	switch {
	case strings.HasSuffix(path, ".html"):
		return "text/html; charset=utf-8"
	case strings.HasSuffix(path, ".js"):
		return "application/javascript; charset=utf-8"
	case strings.HasSuffix(path, ".css"):
		return "text/css; charset=utf-8"
	case strings.HasSuffix(path, ".json"):
		return "application/json; charset=utf-8"
	case strings.HasSuffix(path, ".png"):
		return "image/png"
	case strings.HasSuffix(path, ".jpg"), strings.HasSuffix(path, ".jpeg"):
		return "image/jpeg"
	case strings.HasSuffix(path, ".svg"):
		return "image/svg+xml"
	case strings.HasSuffix(path, ".ico"):
		return "image/x-icon"
	case strings.HasSuffix(path, ".woff"):
		return "font/woff"
	case strings.HasSuffix(path, ".woff2"):
		return "font/woff2"
	case strings.HasSuffix(path, ".ttf"):
		return "font/ttf"
	case strings.HasSuffix(path, ".map"):
		return "application/json"
	default:
		return "application/octet-stream"
	}
}
