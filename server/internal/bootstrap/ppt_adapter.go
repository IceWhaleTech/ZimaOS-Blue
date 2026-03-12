package bootstrap

import (
	"context"
	"errors"
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/i18n"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/mediagen"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/ppt"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

func newPPTService(manager *mediagen.Manager, storage *mediagen.MediaStorage, reviewer *tools.UIReviewerTool) tools.PPTGenerateService {
	if manager == nil || storage == nil {
		return nil
	}
	return pptServiceAdapter{service: ppt.NewService(pptGeneratorAdapter{manager: manager}, storage, pptReviewerAdapter{tool: reviewer})}
}

type pptGeneratorAdapter struct {
	manager *mediagen.Manager
}

func (a pptGeneratorAdapter) HasImageProviders() bool {
	return a.manager != nil && a.manager.HasImageProviders()
}

func (a pptGeneratorAdapter) Models() []mediagen.MediaModelInfo {
	if a.manager == nil {
		return nil
	}
	return a.manager.Models()
}

func (a pptGeneratorAdapter) CreateTask(ctx context.Context, req *mediagen.MediaRequest, messageID, category, source string) (*mediagen.MediaTask, error) {
	if a.manager == nil {
		return nil, errors.New("ppt slide-asset generator is not available")
	}
	return a.manager.CreateTask(ctx, req, messageID, category, source)
}

func (a pptGeneratorAdapter) WaitForTask(ctx context.Context, taskID string) (*mediagen.MediaTask, error) {
	if a.manager == nil {
		return nil, errors.New("ppt slide-asset generator is not available")
	}
	return a.manager.WaitForTask(ctx, taskID)
}

type pptServiceAdapter struct {
	service *ppt.Service
}

func (a pptServiceAdapter) Generate(ctx context.Context, req tools.PPTRequest) (*tools.PPTResult, error) {
	if a.service == nil {
		return nil, errors.New("ppt slide-asset service is not available")
	}
	result, err := a.service.Generate(ctx, ppt.Request{
		Description:       req.Description,
		AspectRatio:       req.AspectRatio,
		ReferenceImages:   append([]string(nil), req.ReferenceImages...),
		StylePreset:       req.StylePreset,
		Theme:             req.Theme,
		Source:            req.Source,
		ReviewThreshold:   req.ReviewThreshold,
		ReviewRetryBudget: req.ReviewRetryBudget,
		QualityProfile:    req.QualityProfile,
		Lang:              req.Lang,
	})
	if result == nil {
		return nil, err
	}
	return &tools.PPTResult{
		Status:         result.Status,
		Skipped:        result.Skipped,
		SkipReason:     result.SkipReason,
		TaskID:         result.TaskID,
		Model:          result.Model,
		Mode:           result.Mode,
		FinalPrompt:    result.FinalPrompt,
		ReviewScore:    result.ReviewScore,
		ReviewSummary:  result.ReviewSummary,
		RetryCount:     result.RetryCount,
		ImageURLs:      append([]string(nil), result.ImageURLs...),
		ThumbnailURLs:  append([]string(nil), result.ThumbnailURLs...),
		Review:         result.Review,
		UsedFallback:   result.UsedFallback,
		QualityProfile: result.QualityProfile,
		StylePreset:    result.StylePreset,
		Source:         result.Source,
		Error:          result.Error,
		Threshold:      result.Threshold,
		Description:    result.Description,
		ReferenceCount: result.ReferenceCount,
	}, err
}

type pptReviewerAdapter struct {
	tool *tools.UIReviewerTool
}

func (a pptReviewerAdapter) ReviewImage(ctx context.Context, imageBase64 string, opts ppt.ReviewOptions) (*ppt.ReviewResult, error) {
	if a.tool == nil {
		return nil, errors.New("ui reviewer tool is not available")
	}
	profile := tools.UIReviewProfileUIScreenshot
	if strings.EqualFold(opts.Profile, ppt.DefaultQualityProfile) {
		profile = tools.UIReviewProfilePPT
	}
	lang := i18n.Language(opts.Lang)
	if lang == "" {
		lang = i18n.LangEnUS
	}
	result, err := a.tool.ReviewImage(ctx, imageBase64, tools.UIReviewImageOptions{
		Threshold: opts.Threshold,
		Format:    "json",
		Lang:      lang,
		Profile:   profile,
	})
	if err != nil {
		return nil, err
	}
	out := &ppt.ReviewResult{
		Overall:     result.Overall,
		Threshold:   result.Threshold,
		Pass:        result.Pass,
		Suggestions: append([]string(nil), result.Suggestions...),
		Human:       result.Human,
	}
	if len(result.Visual.Details) > 0 {
		out.Scores = make(map[string]float64, len(result.Visual.Details))
		for key, value := range result.Visual.Details {
			out.Scores[key] = value
		}
	}
	if len(result.Issues) > 0 {
		out.Issues = make([]ppt.ReviewIssue, 0, len(result.Issues))
		for _, issue := range result.Issues {
			out.Issues = append(out.Issues, ppt.ReviewIssue{
				Severity:    issue.Severity,
				Category:    issue.Category,
				Rule:        issue.Rule,
				Element:     issue.Element,
				Description: issue.Description,
				Location:    issue.Location,
			})
		}
	}
	return out, nil
}
