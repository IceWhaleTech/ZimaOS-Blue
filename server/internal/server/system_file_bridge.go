package server

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/auth"
	convertpkg "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/convert"
	"github.com/labstack/echo/v4"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"

	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/webp"

	_ "image/gif"
	_ "image/png"
)

const (
	defaultThumbnailSize = 256
	minThumbnailSize     = 96
	maxThumbnailSize     = 640
	x2tFixedPath         = "/usr/bin/x2t/x2t"
	x2tPreviewTimeout    = 12 * time.Second
	pdftoppmTimeout      = 12 * time.Second
	ffmpegPreviewTimeout = 12 * time.Second
	textPreviewReadLimit = 64 * 1024
	textPreviewMaxChars  = 84
	textPreviewMaxLines  = 26
)

type revealPathRequest struct {
	Path string `json:"path"`
}

type localFileResolveResponse struct {
	Path         string `json:"path"`
	Name         string `json:"name"`
	SizeBytes    int64  `json:"size_bytes"`
	MimeType     string `json:"mime_type"`
	DownloadURL  string `json:"download_url"`
	ThumbnailURL string `json:"thumbnail_url,omitempty"`
}

func (h *SystemHandler) RevealPath(c echo.Context) error {
	if auth.GetUserFromContext(c) == nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "authentication required")
	}
	if !isLoopbackClientIP(c.RealIP()) {
		return echo.NewHTTPError(http.StatusForbidden, "reveal-path is only available from loopback clients")
	}

	var req revealPathRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request")
	}

	resolvedPath, info, err := resolveLocalPath(req.Path, true)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	if err := revealPathInFileManager(resolvedPath, info.IsDir()); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.JSON(http.StatusOK, map[string]bool{"success": true})
}

func (h *SystemHandler) ResolveLocalFile(c echo.Context) error {
	if auth.GetUserFromContext(c) == nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "authentication required")
	}
	if !isLoopbackClientIP(c.RealIP()) {
		return echo.NewHTTPError(http.StatusForbidden, "local file bridge is only available from loopback clients")
	}

	resolvedPath, info, err := resolveLocalPath(c.QueryParam("path"), false)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	encodedPath := url.QueryEscape(resolvedPath)
	resp := localFileResolveResponse{
		Path:        resolvedPath,
		Name:        filepath.Base(resolvedPath),
		SizeBytes:   info.Size(),
		MimeType:    detectFileMimeType(resolvedPath),
		DownloadURL: "/api/v1/system/local-file/content?path=" + encodedPath,
	}
	if supportsThumbnailPreview(resolvedPath) {
		resp.ThumbnailURL = "/api/v1/system/local-file/thumbnail?path=" + encodedPath
	}

	return c.JSON(http.StatusOK, resp)
}

func (h *SystemHandler) DownloadLocalFile(c echo.Context) error {
	if auth.GetUserFromContext(c) == nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "authentication required")
	}
	if !isLoopbackClientIP(c.RealIP()) {
		return echo.NewHTTPError(http.StatusForbidden, "local file bridge is only available from loopback clients")
	}

	resolvedPath, _, err := resolveLocalPath(c.QueryParam("path"), false)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	if wantsInlineLocalFile(c.QueryParam("inline")) {
		return c.File(resolvedPath)
	}
	return c.Attachment(resolvedPath, filepath.Base(resolvedPath))
}

func (h *SystemHandler) GetLocalFileThumbnail(c echo.Context) error {
	if auth.GetUserFromContext(c) == nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "authentication required")
	}
	if !isLoopbackClientIP(c.RealIP()) {
		return echo.NewHTTPError(http.StatusForbidden, "local file bridge is only available from loopback clients")
	}

	resolvedPath, _, err := resolveLocalPath(c.QueryParam("path"), false)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	src, err := decodePreviewImage(resolvedPath)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "thumbnail preview is not available for this file type")
	}

	size := clampThumbnailSize(c.QueryParam("size"))
	thumb := resizeImageNearest(src, size)

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, thumb, &jpeg.Options{Quality: 78}); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to encode thumbnail")
	}
	c.Response().Header().Set(echo.HeaderCacheControl, "private, max-age=300")
	return c.Blob(http.StatusOK, "image/jpeg", buf.Bytes())
}

func resolveLocalPath(rawPath string, allowDir bool) (string, os.FileInfo, error) {
	trimmed := strings.TrimSpace(rawPath)
	if trimmed == "" {
		return "", nil, errors.New("path is required")
	}
	lower := strings.ToLower(trimmed)
	if strings.HasPrefix(lower, "http://") ||
		strings.HasPrefix(lower, "https://") ||
		strings.HasPrefix(lower, "/api/") ||
		strings.Contains(trimmed, "://") {
		return "", nil, errors.New("path must be a local absolute filesystem path")
	}

	cleanPath := filepath.Clean(trimmed)
	if !filepath.IsAbs(cleanPath) {
		return "", nil, errors.New("path must be absolute")
	}

	info, err := os.Stat(cleanPath)
	if err != nil {
		return "", nil, fmt.Errorf("path not accessible: %w", err)
	}
	if info.IsDir() && !allowDir {
		return "", nil, errors.New("path points to a directory")
	}
	return cleanPath, info, nil
}

func wantsInlineLocalFile(raw string) bool {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "1", "true", "yes", "inline":
		return true
	default:
		return false
	}
}

func revealPathInFileManager(path string, isDir bool) error {
	return revealPathInFileManagerWith(path, isDir, revealPathInFileManagerTarget)
}

func revealPathInFileManagerWith(path string, isDir bool, reveal func(string, bool) error) error {
	err := reveal(path, isDir)
	if err == nil || isDir {
		return err
	}

	parent := parentDirectoryForRevealFallback(path, isDir)
	if parent == "" {
		return err
	}
	if parentErr := reveal(parent, true); parentErr != nil {
		return fmt.Errorf("%v (fallback to parent directory failed: %v)", err, parentErr)
	}
	return nil
}

func parentDirectoryForRevealFallback(path string, isDir bool) string {
	if isDir {
		return ""
	}
	parent := filepath.Dir(path)
	if parent == "" || parent == "." || parent == path {
		return ""
	}
	return parent
}

func revealPathInFileManagerTarget(path string, isDir bool) error {
	switch runtime.GOOS {
	case "darwin":
		if isDir {
			return exec.Command("open", path).Start()
		}
		return exec.Command("open", "-R", path).Start()
	case "windows":
		return revealPathWindows(path, isDir)
	case "linux":
		target := path
		if !isDir {
			parent := filepath.Dir(path)
			if parent != "" {
				target = parent
			}
		}
		return exec.Command("xdg-open", target).Start()
	default:
		return fmt.Errorf("reveal-path is not supported on %s", runtime.GOOS)
	}
}

func isLoopbackClientIP(raw string) bool {
	value := strings.TrimSpace(raw)
	if value == "" {
		return false
	}
	if host, _, err := net.SplitHostPort(value); err == nil {
		value = host
	}
	value = strings.Trim(value, "[]")
	if strings.EqualFold(value, "localhost") {
		return true
	}
	ip := net.ParseIP(value)
	return ip != nil && ip.IsLoopback()
}

func detectFileMimeType(path string) string {
	if ext := strings.ToLower(filepath.Ext(path)); ext != "" {
		if detected := strings.TrimSpace(mime.TypeByExtension(ext)); detected != "" {
			return detected
		}
	}
	return "application/octet-stream"
}

func supportsThumbnailPreview(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".bmp",
		".key", ".pages", ".numbers",
		".docx", ".pptx", ".xlsx", ".docm", ".pptm", ".xlsm",
		".odt", ".odp", ".ods":
		return true
	case ".mp4", ".mov", ".m4v", ".mkv", ".avi", ".webm":
		return resolveVideoThumbnailBinaryPath() != ""
	case ".mp3", ".m4a", ".aac", ".flac", ".ogg", ".opus", ".wav", ".aiff", ".m4b":
		return resolveAudioThumbnailBinaryPath() != ""
	case ".doc", ".ppt", ".xls", ".rtf":
		return resolveX2TBinaryPath() != ""
	case ".pdf":
		return resolvePDFThumbnailBinaryPath() != "" || resolveX2TBinaryPath() != ""
	default:
		return isTextPreviewCandidate(path)
	}
}

func isTextPreviewCandidate(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".txt", ".md", ".markdown", ".json", ".yaml", ".yml", ".toml", ".ini", ".cfg", ".conf",
		".log", ".csv", ".tsv", ".xml", ".html", ".htm", ".js", ".ts", ".jsx", ".tsx",
		".py", ".go", ".rs", ".java", ".c", ".cpp", ".h", ".hpp", ".sh", ".bash", ".zsh",
		".sql", ".env", ".properties":
		return true
	default:
		return false
	}
}

func decodePreviewImage(path string) (image.Image, error) {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".key", ".pages", ".numbers":
		return decodeIWorkQuickLookImage(path)
	case ".docx", ".pptx", ".xlsx", ".docm", ".pptm", ".xlsm":
		img, err := decodeArchiveThumbnailImage(path, []string{
			"docprops/thumbnail.jpg",
			"docprops/thumbnail.jpeg",
			"docprops/thumbnail.png",
		})
		if err == nil {
			return img, nil
		}
		img, x2tErr := decodeWithX2TThumbnail(path)
		if x2tErr == nil {
			return img, nil
		}
		img, textErr := decodeOfficeTextPreviewImage(path)
		if textErr == nil {
			return img, nil
		}
		return nil, errors.Join(x2tErr, textErr)
	case ".odt", ".odp", ".ods":
		img, err := decodeArchiveThumbnailImage(path, []string{
			"thumbnails/thumbnail.png",
			"thumbnails/thumbnail.jpg",
			"thumbnails/thumbnail.jpeg",
		})
		if err == nil {
			return img, nil
		}
		return decodeWithX2TThumbnail(path)
	case ".doc", ".ppt", ".xls", ".rtf":
		return decodeWithX2TThumbnail(path)
	case ".pdf":
		img, err := decodeWithPDFThumbnail(path)
		if err == nil {
			return img, nil
		}
		return decodeWithX2TThumbnail(path)
	case ".mp4", ".mov", ".m4v", ".mkv", ".avi", ".webm":
		return decodeWithVideoThumbnail(path)
	case ".mp3", ".m4a", ".aac", ".flac", ".ogg", ".opus", ".wav", ".aiff", ".m4b":
		return decodeWithAudioThumbnail(path)
	default:
		if isTextPreviewCandidate(path) {
			return decodeTextPreviewImage(path)
		}
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	img, _, err := image.Decode(file)
	if err != nil {
		return nil, err
	}
	return img, nil
}

func decodeIWorkQuickLookImage(path string) (image.Image, error) {
	return decodeArchiveThumbnailImage(path, []string{
		"quicklook/thumbnail.jpg",
		"quicklook/thumbnail.jpeg",
		"quicklook/thumbnail.png",
		"quicklook/preview.jpg",
		"quicklook/preview.jpeg",
		"quicklook/preview.png",
	})
}

func decodeArchiveThumbnailImage(path string, candidates []string) (image.Image, error) {
	reader, err := zip.OpenReader(path)
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	candidateSet := make(map[string]struct{}, len(candidates))
	for _, candidate := range candidates {
		normalized := strings.ToLower(strings.TrimSpace(candidate))
		if normalized == "" {
			continue
		}
		candidateSet[normalized] = struct{}{}
	}

	for _, candidate := range candidates {
		normalizedCandidate := strings.ToLower(strings.TrimSpace(candidate))
		if normalizedCandidate == "" {
			continue
		}
		for _, entry := range reader.File {
			if strings.ToLower(entry.Name) != normalizedCandidate {
				continue
			}
			img, err := decodeArchiveImageEntry(entry)
			if err == nil {
				return img, nil
			}
		}
	}

	for _, entry := range reader.File {
		normalizedName := strings.ToLower(strings.TrimSpace(entry.Name))
		if _, ok := candidateSet[normalizedName]; !ok {
			continue
		}
		img, err := decodeArchiveImageEntry(entry)
		if err == nil {
			return img, nil
		}
	}
	return nil, errors.New("no embedded preview image found in archive")
}

func decodeArchiveImageEntry(entry *zip.File) (image.Image, error) {
	rc, err := entry.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	img, _, err := image.Decode(rc)
	if err != nil {
		return nil, err
	}
	return img, nil
}

func resolveX2TBinaryPath() string {
	if info, err := os.Stat(x2tFixedPath); err == nil && info != nil && !info.IsDir() {
		return x2tFixedPath
	}
	if path, err := exec.LookPath("x2t"); err == nil && strings.TrimSpace(path) != "" {
		return path
	}
	return ""
}

func resolvePDFThumbnailBinaryPath() string {
	if path, err := exec.LookPath("pdftoppm"); err == nil && strings.TrimSpace(path) != "" {
		return path
	}
	return ""
}

func resolveVideoThumbnailBinaryPath() string {
	if path, err := exec.LookPath("ffmpeg"); err == nil && strings.TrimSpace(path) != "" {
		return path
	}
	return ""
}

func resolveAudioThumbnailBinaryPath() string {
	return resolveVideoThumbnailBinaryPath()
}

func decodeWithPDFThumbnail(path string) (image.Image, error) {
	pdftoppmPath := resolvePDFThumbnailBinaryPath()
	if pdftoppmPath == "" {
		return nil, errors.New("pdftoppm is unavailable for pdf thumbnails")
	}

	tmpDir, err := os.MkdirTemp("", "blue-thumb-pdf-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmpDir)

	outputPrefix := filepath.Join(tmpDir, "preview")
	ctx, cancel := context.WithTimeout(context.Background(), pdftoppmTimeout)
	cmd := exec.CommandContext(ctx, pdftoppmPath, "-jpeg", "-singlefile", path, outputPrefix)
	cmdOutput, runErr := cmd.CombinedOutput()
	cancel()
	if runErr != nil {
		msg := strings.TrimSpace(string(cmdOutput))
		if msg != "" {
			return nil, fmt.Errorf("pdftoppm failed: %v (%s)", runErr, msg)
		}
		return nil, fmt.Errorf("pdftoppm failed: %w", runErr)
	}

	candidates := []string{
		outputPrefix + ".jpg",
		outputPrefix + ".jpeg",
		outputPrefix + ".png",
		outputPrefix + "-1.jpg",
		outputPrefix + "-1.jpeg",
		outputPrefix + "-1.png",
	}
	var lastErr error
	for _, candidate := range candidates {
		file, openErr := os.Open(candidate)
		if openErr != nil {
			lastErr = openErr
			continue
		}
		img, _, decodeErr := image.Decode(file)
		_ = file.Close()
		if decodeErr == nil {
			return img, nil
		}
		lastErr = decodeErr
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, errors.New("pdftoppm produced no decodable preview image")
}

func decodeWithVideoThumbnail(path string) (image.Image, error) {
	ffmpegPath := resolveVideoThumbnailBinaryPath()
	if ffmpegPath == "" {
		return nil, errors.New("ffmpeg is unavailable for video thumbnails")
	}

	tmpDir, err := os.MkdirTemp("", "blue-thumb-video-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmpDir)

	outputPath := filepath.Join(tmpDir, "preview.jpg")
	ctx, cancel := context.WithTimeout(context.Background(), ffmpegPreviewTimeout)
	cmd := exec.CommandContext(
		ctx,
		ffmpegPath,
		"-y",
		"-i", path,
		"-frames:v", "1",
		"-q:v", "4",
		outputPath,
	)
	cmdOutput, runErr := cmd.CombinedOutput()
	cancel()
	if runErr != nil {
		msg := strings.TrimSpace(string(cmdOutput))
		if msg != "" {
			return nil, fmt.Errorf("ffmpeg failed: %v (%s)", runErr, msg)
		}
		return nil, fmt.Errorf("ffmpeg failed: %w", runErr)
	}

	file, openErr := os.Open(outputPath)
	if openErr != nil {
		return nil, openErr
	}
	defer file.Close()

	img, _, decodeErr := image.Decode(file)
	if decodeErr != nil {
		return nil, decodeErr
	}
	return img, nil
}

func decodeWithAudioThumbnail(path string) (image.Image, error) {
	ffmpegPath := resolveAudioThumbnailBinaryPath()
	if ffmpegPath == "" {
		return nil, errors.New("ffmpeg is unavailable for audio cover thumbnails")
	}

	tmpDir, err := os.MkdirTemp("", "blue-thumb-audio-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmpDir)

	outputPath := filepath.Join(tmpDir, "cover.jpg")
	attempts := [][]string{
		{"-y", "-i", path, "-map", "0:v:0", "-frames:v", "1", "-q:v", "4", outputPath},
		{"-y", "-i", path, "-an", "-vcodec", "copy", outputPath},
	}

	var lastErr error
	for _, args := range attempts {
		ctx, cancel := context.WithTimeout(context.Background(), ffmpegPreviewTimeout)
		cmd := exec.CommandContext(ctx, ffmpegPath, args...)
		cmdOutput, runErr := cmd.CombinedOutput()
		cancel()
		if runErr != nil {
			msg := strings.TrimSpace(string(cmdOutput))
			if msg != "" {
				lastErr = fmt.Errorf("ffmpeg audio cover extraction failed: %v (%s)", runErr, msg)
			} else {
				lastErr = fmt.Errorf("ffmpeg audio cover extraction failed: %w", runErr)
			}
			continue
		}

		file, openErr := os.Open(outputPath)
		if openErr != nil {
			lastErr = openErr
			continue
		}
		img, _, decodeErr := image.Decode(file)
		_ = file.Close()
		if decodeErr == nil {
			return img, nil
		}
		lastErr = decodeErr
	}

	if lastErr != nil {
		return nil, lastErr
	}
	return nil, errors.New("ffmpeg produced no decodable audio cover image")
}

func decodeTextPreviewImage(path string) (image.Image, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, textPreviewReadLimit))
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		data = []byte("(empty file)")
	}
	if !utf8.Valid(data) {
		data = []byte(strings.ToValidUTF8(string(data), "�"))
	}

	return buildTextPreviewImage(filepath.Base(path), string(data)), nil
}

func decodeOfficeTextPreviewImage(path string) (image.Image, error) {
	reader := convertpkg.NewDocumentReader()
	result, err := reader.ReadDocument(context.Background(), path)
	if err != nil {
		return nil, err
	}
	return buildTextPreviewImage(filepath.Base(path), result.Text), nil
}

func buildTextPreviewImage(filename, rawText string) image.Image {
	if strings.TrimSpace(rawText) == "" {
		rawText = "(empty file)"
	}

	lines := strings.Split(strings.ReplaceAll(rawText, "\r\n", "\n"), "\n")
	renderLines := make([]string, 0, textPreviewMaxLines)
	for _, raw := range lines {
		line := strings.ReplaceAll(raw, "\t", "  ")
		wrapped := wrapTextForPreview(line, textPreviewMaxChars)
		for _, segment := range wrapped {
			if len(renderLines) >= textPreviewMaxLines {
				break
			}
			renderLines = append(renderLines, segment)
		}
		if len(renderLines) >= textPreviewMaxLines {
			break
		}
	}
	if len(renderLines) == 0 {
		renderLines = []string{"(empty file)"}
	}

	const (
		canvasW    = 960
		canvasH    = 640
		headerH    = 44
		paddingX   = 18
		startY     = 64
		lineHeight = 21
	)

	img := image.NewRGBA(image.Rect(0, 0, canvasW, canvasH))
	draw.Draw(img, img.Bounds(), &image.Uniform{C: color.RGBA{R: 246, G: 248, B: 252, A: 255}}, image.Point{}, draw.Src)
	draw.Draw(img, image.Rect(0, 0, canvasW, headerH), &image.Uniform{C: color.RGBA{R: 225, G: 232, B: 242, A: 255}}, image.Point{}, draw.Src)

	face := basicfont.Face7x13
	drawTextLine(img, face, paddingX, 28, filename, color.RGBA{R: 54, G: 63, B: 78, A: 255})

	for i, line := range renderLines {
		y := startY + i*lineHeight
		if y > canvasH-12 {
			break
		}
		drawTextLine(img, face, paddingX, y, line, color.RGBA{R: 33, G: 37, B: 43, A: 255})
	}

	return img
}

func wrapTextForPreview(line string, maxChars int) []string {
	trimmed := strings.TrimRight(line, "\n")
	if trimmed == "" {
		return []string{""}
	}
	runes := []rune(trimmed)
	if len(runes) <= maxChars {
		return []string{trimmed}
	}
	out := make([]string, 0, (len(runes)/maxChars)+1)
	for len(runes) > 0 {
		take := maxChars
		if len(runes) < take {
			take = len(runes)
		}
		segment := string(runes[:take])
		if take == maxChars && len(runes) > take {
			segment += "…"
		}
		out = append(out, segment)
		runes = runes[take:]
	}
	return out
}

func drawTextLine(img *image.RGBA, face font.Face, x, y int, text string, c color.Color) {
	d := &font.Drawer{
		Dst:  img,
		Src:  image.NewUniform(c),
		Face: face,
		Dot:  fixed.P(x, y),
	}
	d.DrawString(text)
}

func decodeWithX2TThumbnail(path string) (image.Image, error) {
	x2tPath := resolveX2TBinaryPath()
	if x2tPath == "" {
		return nil, errors.New("x2t is unavailable for thumbnail conversion")
	}

	tmpDir, err := os.MkdirTemp("", "blue-thumb-x2t-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmpDir)

	outputCandidates := []string{
		filepath.Join(tmpDir, "preview.png"),
		filepath.Join(tmpDir, "preview.jpg"),
		filepath.Join(tmpDir, "preview.jpeg"),
	}

	var lastErr error
	for _, outputPath := range outputCandidates {
		ctx, cancel := context.WithTimeout(context.Background(), x2tPreviewTimeout)
		cmd := exec.CommandContext(ctx, x2tPath, path, outputPath)
		cmdOutput, runErr := cmd.CombinedOutput()
		cancel()

		if runErr != nil {
			msg := strings.TrimSpace(string(cmdOutput))
			if msg != "" {
				lastErr = fmt.Errorf("x2t conversion failed: %v (%s)", runErr, msg)
			} else {
				lastErr = fmt.Errorf("x2t conversion failed: %w", runErr)
			}
			continue
		}

		file, openErr := os.Open(outputPath)
		if openErr != nil {
			lastErr = openErr
			continue
		}
		img, _, decodeErr := image.Decode(file)
		_ = file.Close()
		if decodeErr == nil {
			return img, nil
		}
		lastErr = decodeErr
	}

	if lastErr != nil {
		return nil, lastErr
	}
	return nil, errors.New("x2t did not produce a supported preview image")
}

func clampThumbnailSize(raw string) int {
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || value <= 0 {
		return defaultThumbnailSize
	}
	if value < minThumbnailSize {
		return minThumbnailSize
	}
	if value > maxThumbnailSize {
		return maxThumbnailSize
	}
	return value
}

func resizeImageNearest(src image.Image, maxDim int) image.Image {
	bounds := src.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	if width <= maxDim && height <= maxDim {
		return src
	}

	newW, newH := width, height
	if width >= height {
		newW = maxDim
		newH = height * maxDim / width
	} else {
		newH = maxDim
		newW = width * maxDim / height
	}
	if newW < 1 {
		newW = 1
	}
	if newH < 1 {
		newH = 1
	}

	dst := image.NewRGBA(image.Rect(0, 0, newW, newH))
	for y := 0; y < newH; y++ {
		srcY := y * height / newH
		for x := 0; x < newW; x++ {
			srcX := x * width / newW
			dst.Set(x, y, src.At(bounds.Min.X+srcX, bounds.Min.Y+srcY))
		}
	}
	return dst
}
