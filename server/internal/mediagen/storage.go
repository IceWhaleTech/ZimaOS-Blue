package mediagen

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/network"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
	"github.com/google/uuid"
)

// MediaStorage handles downloading, caching, and serving generated media files.
type MediaStorage struct {
	baseDir string
	baseURL string
	client  *http.Client
}

// NewMediaStorage creates a new media storage.
func NewMediaStorage(baseDir, baseURL string) *MediaStorage {
	return &MediaStorage{
		baseDir: baseDir,
		baseURL: strings.TrimRight(baseURL, "/"),
		client:  network.NewPooledHTTPClient(5 * time.Minute),
	}
}

// EnsureDirs creates the storage directories if they don't exist.
func (s *MediaStorage) EnsureDirs() error {
	for _, sub := range []string{"images", "videos", "thumbnails"} {
		if err := os.MkdirAll(filepath.Join(s.baseDir, sub), 0755); err != nil {
			return err
		}
	}
	return nil
}

// Download fetches a remote URL and stores it locally. Returns the local served URL.
func (s *MediaStorage) Download(ctx context.Context, remoteURL string, mediaType MediaType) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, remoteURL, nil)
	if err != nil {
		return "", err
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download failed: status %d", resp.StatusCode)
	}

	ext := extensionFromContentType(resp.Header.Get("Content-Type"), mediaType)
	filename := uuid.New().String() + ext
	subdir := subdirForType(mediaType)
	localPath := filepath.Join(s.baseDir, subdir, filename)

	f, err := os.Create(localPath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	if _, err := io.Copy(f, resp.Body); err != nil {
		os.Remove(localPath)
		return "", err
	}

	return s.baseURL + "/" + subdir + "/" + filename, nil
}

// StoreBase64 decodes base64 data and stores it locally. Returns the local served URL.
func (s *MediaStorage) StoreBase64(data string, contentType string, mediaType MediaType) (string, error) {
	decoded, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return "", err
	}

	return s.StoreBytes(decoded, contentType, mediaType)
}

// StoreBytes stores raw media bytes locally and returns the served URL.
func (s *MediaStorage) StoreBytes(data []byte, contentType string, mediaType MediaType) (string, error) {
	if s == nil {
		return "", fmt.Errorf("media storage is nil")
	}

	ext := extensionFromContentType(contentType, mediaType)
	filename := uuid.New().String() + ext
	subdir := subdirForType(mediaType)
	localPath := filepath.Join(s.baseDir, subdir, filename)

	if err := os.WriteFile(localPath, data, 0644); err != nil {
		return "", err
	}

	return s.baseURL + "/" + subdir + "/" + filename, nil
}

// Cleanup removes files older than maxAge.
func (s *MediaStorage) Cleanup(maxAge time.Duration) (int, error) {
	cutoff := timeutil.NowTime().Add(-maxAge)
	removed := 0

	for _, subdir := range []string{"images", "videos", "thumbnails"} {
		dir := filepath.Join(s.baseDir, subdir)
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			info, err := entry.Info()
			if err != nil {
				continue
			}
			if info.ModTime().Before(cutoff) {
				os.Remove(filepath.Join(dir, entry.Name()))
				removed++
			}
		}
	}
	return removed, nil
}

// ServeHTTP serves stored media files.
func (s *MediaStorage) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	http.FileServer(http.Dir(s.baseDir)).ServeHTTP(w, r)
}

// ReadServedURL loads a locally cached media asset by its served URL.
func (s *MediaStorage) ReadServedURL(servedURL string) ([]byte, error) {
	if s == nil {
		return nil, fmt.Errorf("media storage is nil")
	}
	return os.ReadFile(s.localPathFromURL(servedURL))
}

// localPathFromURL converts a served URL back to a local file path.
// e.g., "/api/media/generated/images/abc.png" → "{baseDir}/images/abc.png"
func (s *MediaStorage) localPathFromURL(servedURL string) string {
	// Strip the baseURL prefix to get the relative path
	rel := strings.TrimPrefix(servedURL, s.baseURL)
	rel = strings.TrimPrefix(rel, "/")
	return filepath.Join(s.baseDir, rel)
}

func subdirForType(t MediaType) string {
	if t == MediaTypeVideo {
		return "videos"
	}
	return "images"
}

func extensionFromContentType(ct string, t MediaType) string {
	ct = strings.ToLower(ct)
	switch {
	case strings.Contains(ct, "png"):
		return ".png"
	case strings.Contains(ct, "jpeg"), strings.Contains(ct, "jpg"):
		return ".jpg"
	case strings.Contains(ct, "webp"):
		return ".webp"
	case strings.Contains(ct, "gif"):
		return ".gif"
	case strings.Contains(ct, "mp4"):
		return ".mp4"
	case strings.Contains(ct, "webm"):
		return ".webm"
	default:
		if t == MediaTypeVideo {
			return ".mp4"
		}
		return ".png"
	}
}
