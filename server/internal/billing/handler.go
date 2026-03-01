package billing

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/auth"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

const timeFormatRFC3339 = time.RFC3339

// Handler serves billing API endpoints.
type Handler struct {
	service *Service
}

// NewHandler creates a new billing handler.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// RegisterRoutes registers billing endpoints.
func (h *Handler) RegisterRoutes(g *echo.Group) {
	g.GET("/summary", h.GetSummary)
	g.GET("/lines", h.GetLines)
	g.GET("/export", h.Export)
}

// GetSummary handles GET /billing/summary.
func (h *Handler) GetSummary(c echo.Context) error {
	if err := ensureAdmin(c); err != nil {
		return err
	}
	opts, err := parseQueryOptions(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	opts.GroupBy = c.QueryParam("group_by")

	resp, err := h.service.GetSummary(opts)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return c.JSON(http.StatusOK, resp)
}

// GetLines handles GET /billing/lines.
func (h *Handler) GetLines(c echo.Context) error {
	if err := ensureAdmin(c); err != nil {
		return err
	}
	opts, err := parseQueryOptions(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	page := parseIntQuery(c.QueryParam("page"), 1)
	pageSize := parseIntQuery(c.QueryParam("page_size"), defaultPageSize)

	resp, err := h.service.GetLines(opts, page, pageSize)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return c.JSON(http.StatusOK, resp)
}

// Export handles GET /billing/export.
func (h *Handler) Export(c echo.Context) error {
	if err := ensureAdmin(c); err != nil {
		return err
	}
	opts, err := parseQueryOptions(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	format := strings.ToLower(strings.TrimSpace(c.QueryParam("format")))
	if format == "" {
		format = "csv"
	}
	if format != "csv" {
		return echo.NewHTTPError(http.StatusBadRequest, "only csv export is supported")
	}

	data, err := h.service.ExportCSV(opts)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	filename := fmt.Sprintf("billing_%s_%s.csv", opts.Start.Format("20060102"), opts.End.Add(-time.Second).Format("20060102"))
	c.Response().Header().Set("Content-Disposition", "attachment; filename="+filename)
	return c.Blob(http.StatusOK, "text/csv; charset=utf-8", data)
}

func ensureAdmin(c echo.Context) error {
	claims := auth.GetUserFromContext(c)
	if claims == nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "authentication required")
	}
	if strings.ToLower(strings.TrimSpace(claims.Role)) != "admin" {
		return echo.NewHTTPError(http.StatusForbidden, "admin access required")
	}
	return nil
}

func parseQueryOptions(c echo.Context) (QueryOptions, error) {
	start, end, err := parseTimeRange(c.QueryParam("from"), c.QueryParam("to"))
	if err != nil {
		return QueryOptions{}, err
	}
	return QueryOptions{
		Start:      start,
		End:        end,
		ProviderID: strings.TrimSpace(c.QueryParam("provider_id")),
		ModelID:    strings.TrimSpace(c.QueryParam("model_id")),
	}, nil
}

func parseIntQuery(raw string, def int) int {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return def
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return def
	}
	return v
}

func parseTimeRange(fromRaw, toRaw string) (time.Time, time.Time, error) {
	now := timeutil.NowTime().UTC()

	end := now
	if strings.TrimSpace(toRaw) != "" {
		parsed, err := parseTimeParam(toRaw, true)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("invalid to: %w", err)
		}
		end = parsed
	}

	start := end.AddDate(0, 0, -30)
	if strings.TrimSpace(fromRaw) != "" {
		parsed, err := parseTimeParam(fromRaw, false)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("invalid from: %w", err)
		}
		start = parsed
	}

	if !start.Before(end) {
		return time.Time{}, time.Time{}, fmt.Errorf("from must be earlier than to")
	}
	return start, end, nil
}

func parseTimeParam(raw string, isEnd bool) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, fmt.Errorf("empty time")
	}

	if t, err := time.Parse(timeFormatRFC3339, raw); err == nil {
		return t.UTC(), nil
	}
	if t, err := time.ParseInLocation("2006-01-02", raw, time.Local); err == nil {
		if isEnd {
			return t.Add(24 * time.Hour).UTC(), nil
		}
		return t.UTC(), nil
	}
	return time.Time{}, fmt.Errorf("expected RFC3339 or YYYY-MM-DD")
}
