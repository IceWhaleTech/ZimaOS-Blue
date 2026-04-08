package tools

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/ebitengine/purego"
)

const (
	webFetchNativeCurlGlobalDefault int64 = 3

	webFetchNativeCurloptWriteData      uint32 = 10001
	webFetchNativeCurloptURL            uint32 = 10002
	webFetchNativeCurloptWriteFunction  uint32 = 20011
	webFetchNativeCurloptHTTPHeader     uint32 = 10023
	webFetchNativeCurloptHeaderData     uint32 = 10029
	webFetchNativeCurloptFollowLocation uint32 = 52
	webFetchNativeCurloptMaxRedirs      uint32 = 68
	webFetchNativeCurloptHeaderFunction uint32 = 20079
	webFetchNativeCurloptNoSignal       uint32 = 99
	webFetchNativeCurloptAcceptEncoding uint32 = 10102
	webFetchNativeCurloptTimeoutMS      uint32 = 155

	webFetchNativeCurlinfoEffectiveURL uint32 = 0x100000 + 1
	webFetchNativeCurlinfoResponseCode uint32 = 0x200000 + 2
	webFetchNativeCurlinfoContentType  uint32 = 0x100000 + 18
)

var errWebFetchHTTPNativeUnavailable = errors.New("http_native lane is unavailable")

type webFetchHTTPNativeRequest struct {
	URL               string
	Headers           map[string]string
	Timeout           time.Duration
	MaxRedirects      int
	MaxResponseBytes  int64
	AllowPrivateHosts bool
}

type webFetchHTTPNativeResponse struct {
	FinalURL      string
	StatusCode    int
	ContentType   string
	Body          []byte
	BodyTruncated bool
	Headers       http.Header
}

type webFetchHTTPNativeClient interface {
	Available() bool
	Do(context.Context, webFetchHTTPNativeRequest) (webFetchHTTPNativeResponse, error)
}

type libcurlHTTPNativeClient struct {
	enabled bool
	library string

	once    sync.Once
	loadErr error
	lib     uintptr

	globalInit   func(args ...any) int32
	easyInit     func() unsafe.Pointer
	easyCleanup  func(easy unsafe.Pointer)
	easyPerform  func(easy unsafe.Pointer) int32
	easySetopt   func(easy unsafe.Pointer, option uint32, args ...any) int32
	easyGetinfo  func(easy unsafe.Pointer, info uint32, args ...any) int32
	easyStrerror func(code int32) string
	slistAppend  func(list unsafe.Pointer, item *byte) unsafe.Pointer
	slistFreeAll func(list unsafe.Pointer)
}

type webFetchNativeTransferState struct {
	headers       http.Header
	body          []byte
	bodyLimit     int64
	bodyTruncated bool
}

var (
	webFetchNativeTransferSeq atomic.Uint64
	webFetchNativeTransfers   sync.Map
	webFetchNativeWriteHook   = purego.NewCallback(func(_ purego.CDecl, data unsafe.Pointer, size uint64, nmemb uint64, userData unsafe.Pointer) uint64 {
		return webFetchNativeTransferCallback(data, size, nmemb, userData, false)
	})
	webFetchNativeHeaderHook = purego.NewCallback(func(_ purego.CDecl, data unsafe.Pointer, size uint64, nmemb uint64, userData unsafe.Pointer) uint64 {
		return webFetchNativeTransferCallback(data, size, nmemb, userData, true)
	})
)

func newWebFetchHTTPNativeClient(config WebFetchConfig) webFetchHTTPNativeClient {
	if !config.HTTPNativeEnabled {
		return nil
	}
	return &libcurlHTTPNativeClient{
		enabled: true,
		library: strings.TrimSpace(config.HTTPNativeLibrary),
	}
}

func (c *libcurlHTTPNativeClient) Available() bool {
	return c != nil && c.enabled
}

func (c *libcurlHTTPNativeClient) Do(ctx context.Context, req webFetchHTTPNativeRequest) (webFetchHTTPNativeResponse, error) {
	if c == nil || !c.enabled {
		return webFetchHTTPNativeResponse{}, errWebFetchHTTPNativeUnavailable
	}
	if err := c.ensureLoaded(); err != nil {
		return webFetchHTTPNativeResponse{}, err
	}

	normalizedURL, err := normalizeWebFetchURL(req.URL)
	if err != nil {
		return webFetchHTTPNativeResponse{}, err
	}

	currentURL := normalizedURL
	redirects := maxWebQueryInt(req.MaxRedirects, 0)
	for attempt := 0; ; attempt++ {
		if err := guardWebFetchURL(ctx, currentURL, req.AllowPrivateHosts); err != nil {
			return webFetchHTTPNativeResponse{}, err
		}

		timeout := req.Timeout
		if deadline, ok := ctx.Deadline(); ok {
			remaining := time.Until(deadline)
			if timeout <= 0 || remaining < timeout {
				timeout = remaining
			}
		}
		if timeout <= 0 {
			if err := ctx.Err(); err != nil {
				return webFetchHTTPNativeResponse{}, err
			}
			timeout = 30 * time.Second
		}

		resp, err := c.doOnce(ctx, currentURL, req.Headers, timeout, req.MaxResponseBytes)
		if err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return webFetchHTTPNativeResponse{}, ctxErr
			}
			return webFetchHTTPNativeResponse{}, err
		}
		if !shouldFollowWebFetchNativeRedirect(resp.StatusCode) || attempt >= redirects {
			return resp, nil
		}

		location := strings.TrimSpace(resp.Headers.Get("Location"))
		if location == "" {
			return resp, nil
		}
		nextURL, err := resolveWebFetchNativeRedirectURL(currentURL, resp.FinalURL, location)
		if err != nil {
			return resp, nil
		}
		currentURL = nextURL
	}
}

func (c *libcurlHTTPNativeClient) ensureLoaded() error {
	if c == nil {
		return errWebFetchHTTPNativeUnavailable
	}
	c.once.Do(func() {
		c.loadErr = c.load()
	})
	return c.loadErr
}

func (c *libcurlHTTPNativeClient) load() error {
	var lastErr error
	for _, candidate := range c.libraryCandidates() {
		lib, err := webFetchNativeOpenLibrary(candidate)
		if err != nil {
			lastErr = err
			continue
		}
		c.lib = lib
		c.library = candidate
		break
	}
	if c.lib == 0 {
		if lastErr == nil {
			lastErr = errWebFetchHTTPNativeUnavailable
		}
		return fmt.Errorf("load libcurl shared library: %w", lastErr)
	}

	if err := registerWebFetchNativeFunc(c.lib, "curl_global_init", &c.globalInit); err != nil {
		return fmt.Errorf("bind curl_global_init: %w", err)
	}
	if err := registerWebFetchNativeFunc(c.lib, "curl_easy_init", &c.easyInit); err != nil {
		return fmt.Errorf("bind curl_easy_init: %w", err)
	}
	if err := registerWebFetchNativeFunc(c.lib, "curl_easy_cleanup", &c.easyCleanup); err != nil {
		return fmt.Errorf("bind curl_easy_cleanup: %w", err)
	}
	if err := registerWebFetchNativeFunc(c.lib, "curl_easy_perform", &c.easyPerform); err != nil {
		return fmt.Errorf("bind curl_easy_perform: %w", err)
	}
	if err := registerWebFetchNativeFunc(c.lib, "curl_easy_setopt", &c.easySetopt); err != nil {
		return fmt.Errorf("bind curl_easy_setopt: %w", err)
	}
	if err := registerWebFetchNativeFunc(c.lib, "curl_easy_getinfo", &c.easyGetinfo); err != nil {
		return fmt.Errorf("bind curl_easy_getinfo: %w", err)
	}
	if err := registerWebFetchNativeFunc(c.lib, "curl_easy_strerror", &c.easyStrerror); err != nil {
		return fmt.Errorf("bind curl_easy_strerror: %w", err)
	}
	if err := registerWebFetchNativeFunc(c.lib, "curl_slist_append", &c.slistAppend); err != nil {
		return fmt.Errorf("bind curl_slist_append: %w", err)
	}
	if err := registerWebFetchNativeFunc(c.lib, "curl_slist_free_all", &c.slistFreeAll); err != nil {
		return fmt.Errorf("bind curl_slist_free_all: %w", err)
	}

	initCode := c.globalInit(webFetchNativeCLongArg(webFetchNativeCurlGlobalDefault))
	if initCode != 0 {
		return fmt.Errorf("curl_global_init failed: %s", c.errorString(initCode))
	}
	return nil
}

func (c *libcurlHTTPNativeClient) libraryCandidates() []string {
	seen := make(map[string]struct{}, 4)
	out := make([]string, 0, 4)
	appendCandidate := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		if _, ok := seen[value]; ok {
			return
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	appendCandidate(c.library)
	for _, candidate := range defaultWebFetchNativeLibcurlCandidates() {
		appendCandidate(candidate)
	}
	return out
}

func (c *libcurlHTTPNativeClient) doOnce(ctx context.Context, targetURL string, headers map[string]string, timeout time.Duration, bodyLimit int64) (webFetchHTTPNativeResponse, error) {
	easy := c.easyInit()
	if easy == nil {
		return webFetchHTTPNativeResponse{}, errors.New("curl_easy_init returned nil")
	}
	defer c.easyCleanup(easy)

	state := &webFetchNativeTransferState{
		headers:   make(http.Header),
		bodyLimit: bodyLimit,
	}
	if state.bodyLimit <= 0 {
		state.bodyLimit = webFetchDefaultMaxResponseBytes
	}
	token, cleanup := registerWebFetchNativeTransferState(state)
	defer cleanup()

	var keepAlive [][]byte
	if err := c.setoptLong(easy, webFetchNativeCurloptNoSignal, 1); err != nil {
		return webFetchHTTPNativeResponse{}, err
	}
	if err := c.setoptLong(easy, webFetchNativeCurloptFollowLocation, 0); err != nil {
		return webFetchHTTPNativeResponse{}, err
	}
	if err := c.setoptLong(easy, webFetchNativeCurloptMaxRedirs, 0); err != nil {
		return webFetchHTTPNativeResponse{}, err
	}
	if err := c.setoptLong(easy, webFetchNativeCurloptTimeoutMS, int64(timeout/time.Millisecond)); err != nil {
		return webFetchHTTPNativeResponse{}, err
	}

	if err := c.setoptCString(easy, webFetchNativeCurloptURL, targetURL, &keepAlive); err != nil {
		return webFetchHTTPNativeResponse{}, err
	}
	if err := c.setoptCString(easy, webFetchNativeCurloptAcceptEncoding, "", &keepAlive); err != nil {
		return webFetchHTTPNativeResponse{}, err
	}

	if err := c.setoptPointer(easy, webFetchNativeCurloptWriteFunction, unsafe.Pointer(webFetchNativeWriteHook)); err != nil {
		return webFetchHTTPNativeResponse{}, err
	}
	if err := c.setoptPointer(easy, webFetchNativeCurloptWriteData, token); err != nil {
		return webFetchHTTPNativeResponse{}, err
	}

	if err := c.setoptPointer(easy, webFetchNativeCurloptHeaderFunction, unsafe.Pointer(webFetchNativeHeaderHook)); err != nil {
		return webFetchHTTPNativeResponse{}, err
	}
	if err := c.setoptPointer(easy, webFetchNativeCurloptHeaderData, token); err != nil {
		return webFetchHTTPNativeResponse{}, err
	}

	slist, err := c.buildHeaderList(headers)
	if err != nil {
		return webFetchHTTPNativeResponse{}, err
	}
	if slist != nil {
		defer c.slistFreeAll(slist)
		if err := c.setoptPointer(easy, webFetchNativeCurloptHTTPHeader, slist); err != nil {
			return webFetchHTTPNativeResponse{}, err
		}
	}

	performCode := c.easyPerform(easy)
	if performCode != 0 {
		return webFetchHTTPNativeResponse{}, fmt.Errorf("libcurl request failed: %s", c.errorString(performCode))
	}

	resp := webFetchHTTPNativeResponse{
		FinalURL:      targetURL,
		StatusCode:    0,
		ContentType:   strings.TrimSpace(state.headers.Get("Content-Type")),
		Body:          append([]byte(nil), state.body...),
		BodyTruncated: state.bodyTruncated,
		Headers:       cloneWebFetchHTTPHeader(state.headers),
	}

	if statusCode, err := c.getinfoLong(easy, webFetchNativeCurlinfoResponseCode); err == nil {
		resp.StatusCode = int(statusCode)
	}
	if effectiveURL, err := c.getinfoString(easy, webFetchNativeCurlinfoEffectiveURL); err == nil && strings.TrimSpace(effectiveURL) != "" {
		resp.FinalURL = strings.TrimSpace(effectiveURL)
	}
	if contentType, err := c.getinfoString(easy, webFetchNativeCurlinfoContentType); err == nil && strings.TrimSpace(contentType) != "" {
		resp.ContentType = strings.TrimSpace(contentType)
	}

	runtime.KeepAlive(keepAlive)
	if ctxErr := ctx.Err(); ctxErr != nil {
		return webFetchHTTPNativeResponse{}, ctxErr
	}
	return resp, nil
}

func (c *libcurlHTTPNativeClient) buildHeaderList(headers map[string]string) (unsafe.Pointer, error) {
	if len(headers) == 0 {
		return nil, nil
	}
	keys := make([]string, 0, len(headers))
	for key := range headers {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	var slist unsafe.Pointer
	for _, key := range keys {
		value := strings.TrimSpace(headers[key])
		if strings.TrimSpace(key) == "" || value == "" {
			continue
		}
		line := key + ": " + value
		raw := append([]byte(line), 0)
		var item *byte
		if len(raw) > 0 {
			item = &raw[0]
		}
		next := c.slistAppend(slist, item)
		if next == nil {
			if slist != nil {
				c.slistFreeAll(slist)
			}
			return nil, fmt.Errorf("curl_slist_append failed for header %q", key)
		}
		slist = next
		runtime.KeepAlive(raw)
	}
	return slist, nil
}

func (c *libcurlHTTPNativeClient) setoptPointer(easy unsafe.Pointer, option uint32, value unsafe.Pointer) error {
	code := c.easySetopt(easy, option, value)
	if code != 0 {
		return fmt.Errorf("curl_easy_setopt(%d) failed: %s", option, c.errorString(code))
	}
	return nil
}

func (c *libcurlHTTPNativeClient) setoptLong(easy unsafe.Pointer, option uint32, value int64) error {
	code := c.easySetopt(easy, option, webFetchNativeCLongArg(value))
	if code != 0 {
		return fmt.Errorf("curl_easy_setopt(%d) failed: %s", option, c.errorString(code))
	}
	return nil
}

func (c *libcurlHTTPNativeClient) setoptCString(easy unsafe.Pointer, option uint32, value string, keepAlive *[][]byte) error {
	raw := append([]byte(value), 0)
	var ptr unsafe.Pointer
	if len(raw) > 0 {
		ptr = unsafe.Pointer(&raw[0])
	}
	if keepAlive != nil {
		*keepAlive = append(*keepAlive, raw)
	}
	return c.setoptPointer(easy, option, ptr)
}

func (c *libcurlHTTPNativeClient) getinfoLong(easy unsafe.Pointer, info uint32) (int64, error) {
	var (
		code int32
		out  int64
	)
	if runtime.GOOS == "windows" {
		var windowsOut int32
		code = c.easyGetinfo(easy, info, &windowsOut)
		out = int64(windowsOut)
	} else {
		code = c.easyGetinfo(easy, info, &out)
	}
	if code != 0 {
		return 0, fmt.Errorf("curl_easy_getinfo(%d) failed: %s", info, c.errorString(code))
	}
	return out, nil
}

func (c *libcurlHTTPNativeClient) getinfoString(easy unsafe.Pointer, info uint32) (string, error) {
	var ptr *byte
	code := c.easyGetinfo(easy, info, &ptr)
	if code != 0 {
		return "", fmt.Errorf("curl_easy_getinfo(%d) failed: %s", info, c.errorString(code))
	}
	return webFetchNativeCString(ptr), nil
}

func (c *libcurlHTTPNativeClient) errorString(code int32) string {
	if text := strings.TrimSpace(c.easyStrerror(code)); text != "" {
		return text
	}
	return fmt.Sprintf("CURLcode(%d)", code)
}

func registerWebFetchNativeFunc(handle uintptr, name string, out any) (err error) {
	sym, err := webFetchNativeLookupSymbol(handle, name)
	if err != nil {
		return err
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("register %s: %v", name, recovered)
		}
	}()
	purego.RegisterFunc(out, sym)
	return nil
}

func webFetchNativeCLongArg(value int64) any {
	if runtime.GOOS == "windows" {
		return int32(value)
	}
	return value
}

func registerWebFetchNativeTransferState(state *webFetchNativeTransferState) (unsafe.Pointer, func()) {
	id := webFetchNativeTransferSeq.Add(1)
	webFetchNativeTransfers.Store(id, state)
	return unsafe.Pointer(uintptr(id)), func() {
		webFetchNativeTransfers.Delete(id)
	}
}

func webFetchNativeTransferCallback(data unsafe.Pointer, size uint64, nmemb uint64, token unsafe.Pointer, header bool) uint64 {
	total := size * nmemb
	if total == 0 || data == nil {
		return total
	}
	if token == nil {
		return 0
	}

	state, ok := lookupWebFetchNativeTransferState(token)
	if !ok || state == nil {
		return 0
	}

	chunk := unsafe.Slice((*byte)(data), int(total))
	if header {
		state.addHeaderLine(chunk)
	} else {
		state.appendBody(chunk)
	}
	return total
}

func lookupWebFetchNativeTransferState(token unsafe.Pointer) (*webFetchNativeTransferState, bool) {
	id := uint64(uintptr(token))
	if id == 0 {
		return nil, false
	}
	raw, ok := webFetchNativeTransfers.Load(id)
	if !ok {
		return nil, false
	}
	state, ok := raw.(*webFetchNativeTransferState)
	return state, ok
}

func (s *webFetchNativeTransferState) appendBody(chunk []byte) {
	if s == nil || len(chunk) == 0 {
		return
	}
	if s.bodyLimit <= 0 {
		s.body = append(s.body, chunk...)
		return
	}
	remaining := s.bodyLimit - int64(len(s.body))
	if remaining <= 0 {
		s.bodyTruncated = true
		return
	}
	if int64(len(chunk)) > remaining {
		s.body = append(s.body, chunk[:remaining]...)
		s.bodyTruncated = true
		return
	}
	s.body = append(s.body, chunk...)
}

func (s *webFetchNativeTransferState) addHeaderLine(chunk []byte) {
	if s == nil || len(chunk) == 0 {
		return
	}
	line := strings.TrimRight(string(chunk), "\r\n")
	if line == "" {
		return
	}
	if strings.HasPrefix(strings.ToUpper(line), "HTTP/") {
		return
	}
	idx := strings.IndexByte(line, ':')
	if idx <= 0 {
		return
	}
	name := http.CanonicalHeaderKey(strings.TrimSpace(line[:idx]))
	value := strings.TrimSpace(line[idx+1:])
	if name == "" {
		return
	}
	s.headers.Add(name, value)
}

func shouldFollowWebFetchNativeRedirect(statusCode int) bool {
	switch statusCode {
	case http.StatusMovedPermanently,
		http.StatusFound,
		http.StatusSeeOther,
		http.StatusTemporaryRedirect,
		http.StatusPermanentRedirect:
		return true
	default:
		return false
	}
}

func resolveWebFetchNativeRedirectURL(currentURL, finalURL, location string) (string, error) {
	base := strings.TrimSpace(finalURL)
	if base == "" {
		base = strings.TrimSpace(currentURL)
	}
	baseURL, err := url.Parse(base)
	if err != nil {
		return "", err
	}
	ref, err := url.Parse(strings.TrimSpace(location))
	if err != nil {
		return "", err
	}
	return baseURL.ResolveReference(ref).String(), nil
}

func cloneWebFetchHTTPHeader(src http.Header) http.Header {
	if len(src) == 0 {
		return nil
	}
	dst := make(http.Header, len(src))
	for key, values := range src {
		dst[key] = append([]string(nil), values...)
	}
	return dst
}

func defaultWebFetchNativeLibcurlCandidates() []string {
	switch runtime.GOOS {
	case "darwin":
		return []string{"/usr/lib/libcurl.4.dylib", "libcurl.4.dylib", "libcurl.dylib"}
	case "linux", "freebsd":
		return []string{"libcurl.so.4", "libcurl.so"}
	case "windows":
		return []string{"libcurl.dll", "curl.dll"}
	default:
		return nil
	}
}

func webFetchNativeCString(ptr *byte) string {
	if ptr == nil {
		return ""
	}
	length := 0
	for {
		if *(*byte)(unsafe.Pointer(uintptr(unsafe.Pointer(ptr)) + uintptr(length))) == 0 {
			break
		}
		length++
	}
	if length == 0 {
		return ""
	}
	return string(unsafe.Slice(ptr, length))
}
