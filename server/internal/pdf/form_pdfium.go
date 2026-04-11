package pdf

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/klippa-app/go-pdfium"
	"github.com/klippa-app/go-pdfium/enums"
	"github.com/klippa-app/go-pdfium/references"
	"github.com/klippa-app/go-pdfium/requests"
	"github.com/klippa-app/go-pdfium/structs"
	"github.com/klippa-app/go-pdfium/webassembly"
)

type pdfFormFillHost struct {
	instance    pdfium.Pdfium
	document    references.FPDF_DOCUMENT
	pageRefs    map[int]references.FPDF_PAGE
	currentPage int
	nextTimerID int
}

func newPDFFormFillHost(instance pdfium.Pdfium, document references.FPDF_DOCUMENT) *pdfFormFillHost {
	return &pdfFormFillHost{
		instance:    instance,
		document:    document,
		pageRefs:    make(map[int]references.FPDF_PAGE),
		currentPage: -1,
		nextTimerID: 1,
	}
}

func (h *pdfFormFillHost) loadPage(index int) *references.FPDF_PAGE {
	if h == nil || h.instance == nil || h.document == "" || index < 0 {
		return nil
	}
	if page, ok := h.pageRefs[index]; ok {
		pageCopy := page
		return &pageCopy
	}
	loaded, err := h.instance.FPDF_LoadPage(&requests.FPDF_LoadPage{Document: h.document, Index: index})
	if err != nil || loaded == nil || loaded.Page == "" {
		return nil
	}
	h.pageRefs[index] = loaded.Page
	pageCopy := loaded.Page
	return &pageCopy
}

func (h *pdfFormFillHost) closeAllPages() {
	if h == nil || h.instance == nil {
		return
	}
	indexes := make([]int, 0, len(h.pageRefs))
	for index := range h.pageRefs {
		indexes = append(indexes, index)
	}
	sort.Ints(indexes)
	for _, index := range indexes {
		page := h.pageRefs[index]
		if page == "" {
			continue
		}
		_, _ = h.instance.FPDF_ClosePage(&requests.FPDF_ClosePage{Page: page})
	}
}

func (h *pdfFormFillHost) formFillInfo() structs.FPDF_FORMFILLINFO {
	return structs.FPDF_FORMFILLINFO{
		Release:                func() {},
		FFI_Invalidate:         func(references.FPDF_PAGE, float64, float64, float64, float64) {},
		FFI_OutputSelectedRect: func(references.FPDF_PAGE, float64, float64, float64, float64) {},
		FFI_SetCursor:          func(enums.FXCT) {},
		FFI_KillTimer:          func(int) {},
		FFI_OnChange:           func() {},
		FFI_ExecuteNamedAction: func(string) {},
		FFI_SetTextFieldFocus:  func(string, bool) {},
		FFI_DoURIAction:        func(string) {},
		FFI_DoGoToAction:       func(int, enums.FPDF_ZOOM_MODE, []float32) {},
		FFI_GetRotation:        func(references.FPDF_PAGE) enums.FPDF_PAGE_ROTATION { return enums.FPDF_PAGE_ROTATION_NONE },
		FFI_SetTimer:           h.setTimer,
		FFI_GetLocalTime:       currentPDFSystemTime,
		FFI_GetPage:            h.getPage,
		FFI_GetCurrentPage:     h.getCurrentPage,
	}
}

func (h *pdfFormFillHost) setTimer(_ int, _ func(idEvent int)) int {
	if h == nil {
		return 0
	}
	id := h.nextTimerID
	h.nextTimerID++
	return id
}

func (h *pdfFormFillHost) getPage(document references.FPDF_DOCUMENT, index int) *references.FPDF_PAGE {
	if h == nil || document != h.document {
		return nil
	}
	return h.loadPage(index)
}

func (h *pdfFormFillHost) getCurrentPage(document references.FPDF_DOCUMENT) *references.FPDF_PAGE {
	if h == nil || document != h.document || h.currentPage < 0 {
		return nil
	}
	return h.loadPage(h.currentPage)
}

func currentPDFSystemTime() structs.FPDF_SYSTEMTIME {
	now := time.Now()
	return structs.FPDF_SYSTEMTIME{
		Year:         uint16(now.Year() - 1900),
		Month:        uint16(now.Month() - 1),
		DayOfWeek:    uint16(now.Weekday()),
		Day:          uint16(now.Day()),
		Hour:         uint16(now.Hour()),
		Minute:       uint16(now.Minute()),
		Second:       uint16(now.Second()),
		Milliseconds: uint16(now.Nanosecond() / int(time.Millisecond)),
	}
}

type pdfFormWidget struct {
	PageIndex  int
	PageNumber int
	Page       requests.Page
	Annotation references.FPDF_ANNOTATION
	Field      FormField
}

// InspectForm reads interactive PDF form metadata through pdfium/webassembly.
func (s *Service) InspectForm(ctx context.Context, path string) (FormInspectResult, error) {
	if err := ctx.Err(); err != nil {
		return FormInspectResult{}, err
	}
	resolvedPath, stat, err := resolvePath(path)
	if err != nil {
		return FormInspectResult{}, err
	}
	wasmBytes, err := s.ensureRuntimeWASMBytes(ctx)
	if err != nil {
		return FormInspectResult{}, err
	}
	pool, err := webassembly.Init(webassembly.Config{
		MinIdle:      1,
		MaxIdle:      1,
		MaxTotal:     1,
		ReuseWorkers: true,
		Stdout:       io.Discard,
		Stderr:       io.Discard,
		WASM:         wasmBytes,
	})
	if err != nil {
		return FormInspectResult{}, fmt.Errorf("init pdfium: %w", err)
	}
	defer func() {
		_ = pool.Close()
	}()

	instance, err := pool.GetInstance(instanceAcquireTimeout)
	if err != nil {
		return FormInspectResult{}, fmt.Errorf("acquire pdfium instance: %w", err)
	}
	defer instance.Close()

	doc, err := openDocument(instance, resolvedPath)
	if err != nil {
		return FormInspectResult{}, err
	}
	defer closeDocument(instance, doc.Document)

	info, err := buildDocumentInfo(instance, resolvedPath, stat, doc.Document)
	if err != nil {
		return FormInspectResult{}, err
	}

	host := newPDFFormFillHost(instance, doc.Document)
	formEnv, err := instance.FPDFDOC_InitFormFillEnvironment(&requests.FPDFDOC_InitFormFillEnvironment{
		Document:     doc.Document,
		FormFillInfo: host.formFillInfo(),
	})
	if err != nil {
		return FormInspectResult{}, fmt.Errorf("init form fill environment: %w", err)
	}
	defer func() {
		_, _ = instance.FPDFDOC_ExitFormFillEnvironment(&requests.FPDFDOC_ExitFormFillEnvironment{FormHandle: formEnv.FormHandle})
		host.closeAllPages()
	}()

	formTypeResp, err := instance.FPDF_GetFormType(&requests.FPDF_GetFormType{Document: doc.Document})
	if err != nil {
		return FormInspectResult{}, fmt.Errorf("get form type: %w", err)
	}
	result := FormInspectResult{
		Document: info,
		FormType: pdfFormTypeName(formTypeResp.FormType),
	}
	if formTypeResp.FormType == enums.FPDF_FORMTYPE_NONE {
		return result, nil
	}
	if formTypeResp.FormType == enums.FPDF_FORMTYPE_XFA_FULL || formTypeResp.FormType == enums.FPDF_FORMTYPE_XFA_FOREGROUND {
		result.Warnings = append(result.Warnings, "XFA forms may expose incomplete field metadata in the current native fill implementation")
	}

	for pageIndex := 0; pageIndex < info.PageCount; pageIndex++ {
		pageRef := host.loadPage(pageIndex)
		if pageRef == nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("page %d could not be loaded for form inspection", pageIndex+1))
			continue
		}
		host.currentPage = pageIndex
		page := requests.Page{ByReference: pageRef}

		if _, err := instance.FORM_OnAfterLoadPage(&requests.FORM_OnAfterLoadPage{Page: page, FormHandle: formEnv.FormHandle}); err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("page %d form initialization failed: %v", pageIndex+1, err))
		}

		annotCount, err := instance.FPDFPage_GetAnnotCount(&requests.FPDFPage_GetAnnotCount{Page: page})
		if err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("page %d annotation count failed: %v", pageIndex+1, err))
			_, _ = instance.FORM_OnBeforeClosePage(&requests.FORM_OnBeforeClosePage{Page: page, FormHandle: formEnv.FormHandle})
			continue
		}

		for annotIndex := 0; annotIndex < annotCount.Count; annotIndex++ {
			annot, err := instance.FPDFPage_GetAnnot(&requests.FPDFPage_GetAnnot{Page: page, Index: annotIndex})
			if err != nil || annot == nil || annot.Annotation == "" {
				result.Warnings = append(result.Warnings, fmt.Sprintf("page %d annotation %d could not be loaded", pageIndex+1, annotIndex))
				continue
			}

			field, ok, warnings := inspectPDFFormAnnotation(instance, formEnv.FormHandle, annot.Annotation, pageIndex+1)
			result.Warnings = append(result.Warnings, warnings...)
			if ok {
				result.Fields = append(result.Fields, field)
			}
			_, _ = instance.FPDFPage_CloseAnnot(&requests.FPDFPage_CloseAnnot{Annotation: annot.Annotation})
		}

		_, _ = instance.FORM_OnBeforeClosePage(&requests.FORM_OnBeforeClosePage{Page: page, FormHandle: formEnv.FormHandle})
	}

	result.FieldCount = len(result.Fields)
	result.Warnings = compactFormWarnings(result.Warnings)
	return result, nil
}

// FillForm updates supported AcroForm widgets and returns the saved document bytes.
func (s *Service) FillForm(ctx context.Context, req FillFormRequest) (FillFormResult, error) {
	if err := ctx.Err(); err != nil {
		return FillFormResult{}, err
	}
	if strings.TrimSpace(req.Path) == "" {
		return FillFormResult{}, fmt.Errorf("pdf fill requires a source path")
	}
	if len(req.Fields) == 0 {
		return FillFormResult{}, fmt.Errorf("pdf fill requires at least one field value")
	}

	resolvedPath, stat, err := resolvePath(req.Path)
	if err != nil {
		return FillFormResult{}, err
	}
	wasmBytes, err := s.ensureRuntimeWASMBytes(ctx)
	if err != nil {
		return FillFormResult{}, err
	}
	pool, err := webassembly.Init(webassembly.Config{
		MinIdle:      1,
		MaxIdle:      1,
		MaxTotal:     1,
		ReuseWorkers: true,
		Stdout:       io.Discard,
		Stderr:       io.Discard,
		WASM:         wasmBytes,
	})
	if err != nil {
		return FillFormResult{}, fmt.Errorf("init pdfium: %w", err)
	}
	defer func() {
		_ = pool.Close()
	}()

	instance, err := pool.GetInstance(instanceAcquireTimeout)
	if err != nil {
		return FillFormResult{}, fmt.Errorf("acquire pdfium instance: %w", err)
	}
	defer instance.Close()

	doc, err := openDocument(instance, resolvedPath)
	if err != nil {
		return FillFormResult{}, err
	}
	defer closeDocument(instance, doc.Document)

	info, err := buildDocumentInfo(instance, resolvedPath, stat, doc.Document)
	if err != nil {
		return FillFormResult{}, err
	}

	host := newPDFFormFillHost(instance, doc.Document)
	formEnv, err := instance.FPDFDOC_InitFormFillEnvironment(&requests.FPDFDOC_InitFormFillEnvironment{
		Document:     doc.Document,
		FormFillInfo: host.formFillInfo(),
	})
	if err != nil {
		return FillFormResult{}, fmt.Errorf("init form fill environment: %w", err)
	}

	widgets := make([]pdfFormWidget, 0, 8)
	defer func() {
		closePDFFormWidgets(instance, widgets)
		beforeClosePDFFormPages(instance, host, formEnv.FormHandle)
		_, _ = instance.FPDFDOC_ExitFormFillEnvironment(&requests.FPDFDOC_ExitFormFillEnvironment{FormHandle: formEnv.FormHandle})
		host.closeAllPages()
	}()

	formTypeResp, err := instance.FPDF_GetFormType(&requests.FPDF_GetFormType{Document: doc.Document})
	if err != nil {
		return FillFormResult{}, fmt.Errorf("get form type: %w", err)
	}
	if formTypeResp.FormType == enums.FPDF_FORMTYPE_NONE {
		return FillFormResult{}, fmt.Errorf("pdf does not contain an interactive form")
	}
	if formTypeResp.FormType != enums.FPDF_FORMTYPE_ACRO_FORM {
		return FillFormResult{}, fmt.Errorf("pdf fill currently supports AcroForm text fields, combo boxes, list boxes, checkboxes, and radio buttons only")
	}

	widgets, warnings := collectPDFFormWidgets(instance, host, formEnv.FormHandle, info)
	groups := groupPDFFormWidgets(widgets)
	updatedFields := make([]string, 0, len(req.Fields))
	for _, fieldName := range sortedPDFFieldNames(req.Fields) {
		candidates, ok := groups[fieldName]
		if !ok || len(candidates) == 0 {
			return FillFormResult{}, fmt.Errorf("requested pdf form field not found: %s", fieldName)
		}
		target := candidates[0]
		if target.Field.Type == "radio" {
			target, err = resolvePDFRadioWidget(candidates, req.Fields[fieldName])
			if err != nil {
				return FillFormResult{}, fmt.Errorf("set radio selection for pdf form field %q: %w", fieldName, err)
			}
		}
		if target.Field.ReadOnly {
			return FillFormResult{}, fmt.Errorf("pdf form field %q is read-only", fieldName)
		}
		host.currentPage = target.PageIndex
		if err := focusPDFFormWidget(instance, formEnv.FormHandle, target); err != nil {
			return FillFormResult{}, fmt.Errorf("focus pdf form field %q: %w", fieldName, err)
		}

		switch target.Field.Type {
		case "text":
			if _, err := instance.FORM_SelectAllText(&requests.FORM_SelectAllText{
				FormHandle: formEnv.FormHandle,
				Page:       target.Page,
			}); err != nil {
				return FillFormResult{}, fmt.Errorf("select text for pdf form field %q: %w", fieldName, err)
			}
			if _, err := instance.FORM_ReplaceSelection(&requests.FORM_ReplaceSelection{
				FormHandle: formEnv.FormHandle,
				Page:       target.Page,
				Text:       req.Fields[fieldName],
			}); err != nil {
				return FillFormResult{}, fmt.Errorf("replace text for pdf form field %q: %w", fieldName, err)
			}
		case "combo", "list":
			optionIndex, err := resolvePDFChoiceOptionIndex(target.Field, req.Fields[fieldName])
			if err != nil {
				return FillFormResult{}, fmt.Errorf("select %s option for pdf form field %q: %w", target.Field.Type, fieldName, err)
			}
			if _, err := instance.FORM_SetIndexSelected(&requests.FORM_SetIndexSelected{
				FormHandle: formEnv.FormHandle,
				Page:       target.Page,
				Index:      optionIndex,
				Selected:   true,
			}); err != nil {
				return FillFormResult{}, fmt.Errorf("set %s option for pdf form field %q: %w", target.Field.Type, fieldName, err)
			}
		case "checkbox":
			desiredChecked, err := resolvePDFCheckboxState(target.Field, req.Fields[fieldName])
			if err != nil {
				return FillFormResult{}, fmt.Errorf("set checkbox state for pdf form field %q: %w", fieldName, err)
			}
			if target.Field.Checked != desiredChecked {
				if err := clickPDFFormWidget(instance, formEnv.FormHandle, target, "checkbox"); err != nil {
					return FillFormResult{}, fmt.Errorf("toggle checkbox for pdf form field %q: %w", fieldName, err)
				}
			}
		case "radio":
			if !target.Field.Checked {
				if err := clickPDFFormWidget(instance, formEnv.FormHandle, target, "radio"); err != nil {
					return FillFormResult{}, fmt.Errorf("select radio option for pdf form field %q: %w", fieldName, err)
				}
			}
		default:
			return FillFormResult{}, fmt.Errorf("pdf fill currently supports text fields, combo boxes, list boxes, checkboxes, and radio buttons only; field %q has type %q", fieldName, target.Field.Type)
		}
		updatedFields = append(updatedFields, fieldName)
	}

	if _, err := instance.FORM_ForceToKillFocus(&requests.FORM_ForceToKillFocus{FormHandle: formEnv.FormHandle}); err != nil {
		warnings = append(warnings, fmt.Sprintf("commit focused pdf form field edits: %v", err))
	}

	saved, err := instance.FPDF_SaveAsCopy(&requests.FPDF_SaveAsCopy{
		Document: doc.Document,
		Flags:    requests.SaveFlagNoIncremental,
	})
	if err != nil {
		return FillFormResult{}, fmt.Errorf("save filled pdf: %w", err)
	}
	if saved == nil || saved.FileBytes == nil || len(*saved.FileBytes) == 0 {
		return FillFormResult{}, fmt.Errorf("save filled pdf: empty output")
	}

	return FillFormResult{
		Document:      info,
		Bytes:         append([]byte(nil), (*saved.FileBytes)...),
		UpdatedFields: updatedFields,
		Warnings:      compactFormWarnings(warnings),
	}, nil
}

func focusPDFFormWidget(instance pdfium.Pdfium, formHandle references.FPDF_FORMHANDLE, target pdfFormWidget) error {
	if _, err := instance.FORM_SetFocusedAnnot(&requests.FORM_SetFocusedAnnot{
		FormHandle: formHandle,
		Annotation: target.Annotation,
	}); err != nil {
		return err
	}
	if target.Field.Rect != nil {
		if _, err := instance.FORM_OnFocus(&requests.FORM_OnFocus{
			FormHandle: formHandle,
			Page:       target.Page,
			PageX:      (target.Field.Rect.Left + target.Field.Rect.Right) / 2,
			PageY:      (target.Field.Rect.Top + target.Field.Rect.Bottom) / 2,
		}); err != nil {
			return err
		}
	}
	return nil
}

func resolvePDFChoiceOptionIndex(field FormField, requested string) (int, error) {
	requested = strings.TrimSpace(requested)
	if requested == "" {
		return 0, fmt.Errorf("requested option cannot be empty")
	}
	for _, option := range field.Options {
		if strings.TrimSpace(option.Label) == requested {
			return option.Index, nil
		}
	}
	if idx, err := strconv.Atoi(requested); err == nil {
		for _, option := range field.Options {
			if option.Index == idx {
				return idx, nil
			}
		}
	}
	available := make([]string, 0, len(field.Options))
	for _, option := range field.Options {
		label := strings.TrimSpace(option.Label)
		if label == "" {
			label = fmt.Sprintf("%d", option.Index)
		} else {
			label = fmt.Sprintf("%d:%s", option.Index, label)
		}
		available = append(available, label)
	}
	return 0, fmt.Errorf("unsupported option %q (available: %s)", requested, strings.Join(available, ", "))
}

func resolvePDFCheckboxState(field FormField, requested string) (bool, error) {
	requested = strings.TrimSpace(requested)
	switch strings.ToLower(requested) {
	case "1", "true", "yes", "on", "checked":
		return true, nil
	case "0", "false", "no", "off", "unchecked":
		return false, nil
	}
	if exportValue := strings.TrimSpace(field.ExportValue); exportValue != "" && strings.EqualFold(exportValue, requested) {
		return true, nil
	}
	return false, fmt.Errorf("unsupported checkbox value %q", requested)
}

func resolvePDFRadioWidget(candidates []pdfFormWidget, requested string) (pdfFormWidget, error) {
	requested = strings.TrimSpace(requested)
	if requested == "" {
		return pdfFormWidget{}, fmt.Errorf("requested export value cannot be empty")
	}

	available := make([]string, 0, len(candidates))
	seen := make(map[string]struct{}, len(candidates))
	for _, candidate := range candidates {
		exportValue := strings.TrimSpace(candidate.Field.ExportValue)
		if exportValue != "" {
			if _, ok := seen[exportValue]; !ok {
				seen[exportValue] = struct{}{}
				available = append(available, exportValue)
			}
			if strings.EqualFold(exportValue, requested) {
				return candidate, nil
			}
		}
	}

	return pdfFormWidget{}, fmt.Errorf("unsupported export value %q (available: %s)", requested, strings.Join(available, ", "))
}

func clickPDFFormWidget(instance pdfium.Pdfium, formHandle references.FPDF_FORMHANDLE, target pdfFormWidget, widgetType string) error {
	if target.Field.Rect == nil {
		return fmt.Errorf("%s rect metadata unavailable", widgetType)
	}
	pageX := (target.Field.Rect.Left + target.Field.Rect.Right) / 2
	pageY := (target.Field.Rect.Top + target.Field.Rect.Bottom) / 2
	if _, err := instance.FORM_OnLButtonDown(&requests.FORM_OnLButtonDown{
		FormHandle: formHandle,
		Page:       target.Page,
		PageX:      pageX,
		PageY:      pageY,
	}); err != nil {
		return err
	}
	if _, err := instance.FORM_OnLButtonUp(&requests.FORM_OnLButtonUp{
		FormHandle: formHandle,
		Page:       target.Page,
		PageX:      pageX,
		PageY:      pageY,
	}); err != nil {
		return err
	}
	return nil
}

func collectPDFFormWidgets(instance pdfium.Pdfium, host *pdfFormFillHost, formHandle references.FPDF_FORMHANDLE, info DocumentInfo) ([]pdfFormWidget, []string) {
	widgets := make([]pdfFormWidget, 0, info.PageCount)
	warnings := make([]string, 0, 4)
	for pageIndex := 0; pageIndex < info.PageCount; pageIndex++ {
		pageRef := host.loadPage(pageIndex)
		if pageRef == nil {
			warnings = append(warnings, fmt.Sprintf("page %d could not be loaded for form inspection", pageIndex+1))
			continue
		}
		host.currentPage = pageIndex
		page := requests.Page{ByReference: pageRef}

		if _, err := instance.FORM_OnAfterLoadPage(&requests.FORM_OnAfterLoadPage{Page: page, FormHandle: formHandle}); err != nil {
			warnings = append(warnings, fmt.Sprintf("page %d form initialization failed: %v", pageIndex+1, err))
		}

		annotCount, err := instance.FPDFPage_GetAnnotCount(&requests.FPDFPage_GetAnnotCount{Page: page})
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("page %d annotation count failed: %v", pageIndex+1, err))
			continue
		}

		for annotIndex := 0; annotIndex < annotCount.Count; annotIndex++ {
			annot, err := instance.FPDFPage_GetAnnot(&requests.FPDFPage_GetAnnot{Page: page, Index: annotIndex})
			if err != nil || annot == nil || annot.Annotation == "" {
				warnings = append(warnings, fmt.Sprintf("page %d annotation %d could not be loaded", pageIndex+1, annotIndex))
				continue
			}

			field, ok, widgetWarnings := inspectPDFFormAnnotation(instance, formHandle, annot.Annotation, pageIndex+1)
			warnings = append(warnings, widgetWarnings...)
			if !ok {
				_, _ = instance.FPDFPage_CloseAnnot(&requests.FPDFPage_CloseAnnot{Annotation: annot.Annotation})
				continue
			}

			widgets = append(widgets, pdfFormWidget{
				PageIndex:  pageIndex,
				PageNumber: pageIndex + 1,
				Page:       page,
				Annotation: annot.Annotation,
				Field:      field,
			})
		}
	}
	return widgets, compactFormWarnings(warnings)
}

func closePDFFormWidgets(instance pdfium.Pdfium, widgets []pdfFormWidget) {
	for _, widget := range widgets {
		if widget.Annotation == "" {
			continue
		}
		_, _ = instance.FPDFPage_CloseAnnot(&requests.FPDFPage_CloseAnnot{Annotation: widget.Annotation})
	}
}

func beforeClosePDFFormPages(instance pdfium.Pdfium, host *pdfFormFillHost, formHandle references.FPDF_FORMHANDLE) {
	if host == nil {
		return
	}
	indexes := make([]int, 0, len(host.pageRefs))
	for index := range host.pageRefs {
		indexes = append(indexes, index)
	}
	sort.Ints(indexes)
	for _, index := range indexes {
		page := host.pageRefs[index]
		if page == "" {
			continue
		}
		_, _ = instance.FORM_OnBeforeClosePage(&requests.FORM_OnBeforeClosePage{
			Page:       requests.Page{ByReference: &page},
			FormHandle: formHandle,
		})
	}
}

func groupPDFFormWidgets(widgets []pdfFormWidget) map[string][]pdfFormWidget {
	groups := make(map[string][]pdfFormWidget, len(widgets))
	for _, widget := range widgets {
		if name := strings.TrimSpace(widget.Field.Name); name != "" {
			groups[name] = append(groups[name], widget)
		}
		if alt := strings.TrimSpace(widget.Field.AlternateName); alt != "" {
			if alt != strings.TrimSpace(widget.Field.Name) {
				groups[alt] = append(groups[alt], widget)
			}
		}
	}
	return groups
}

func sortedPDFFieldNames(values map[string]string) []string {
	names := make([]string, 0, len(values))
	for name := range values {
		trimmed := strings.TrimSpace(name)
		if trimmed == "" {
			continue
		}
		names = append(names, trimmed)
	}
	sort.Strings(names)
	return names
}

func inspectPDFFormAnnotation(instance pdfium.Pdfium, formHandle references.FPDF_FORMHANDLE, annotation references.FPDF_ANNOTATION, pageNumber int) (FormField, bool, []string) {
	subtype, err := instance.FPDFAnnot_GetSubtype(&requests.FPDFAnnot_GetSubtype{Annotation: annotation})
	if err != nil {
		return FormField{}, false, []string{fmt.Sprintf("page %d annotation subtype lookup failed: %v", pageNumber, err)}
	}
	if subtype.Subtype != enums.FPDF_ANNOT_SUBTYPE_WIDGET && subtype.Subtype != enums.FPDF_ANNOT_SUBTYPE_XFAWIDGET {
		return FormField{}, false, nil
	}

	field := FormField{PageNumber: pageNumber, Type: "unknown"}
	warnings := make([]string, 0, 2)

	if resp, err := instance.FPDFAnnot_GetFormFieldName(&requests.FPDFAnnot_GetFormFieldName{FormHandle: formHandle, Annotation: annotation}); err == nil {
		field.Name = strings.TrimSpace(resp.FormFieldName)
	} else {
		warnings = append(warnings, fmt.Sprintf("page %d widget name lookup failed: %v", pageNumber, err))
	}
	if resp, err := instance.FPDFAnnot_GetFormFieldAlternateName(&requests.FPDFAnnot_GetFormFieldAlternateName{FormHandle: formHandle, Annotation: annotation}); err == nil {
		field.AlternateName = strings.TrimSpace(resp.FormFieldAlternateName)
	}
	if resp, err := instance.FPDFAnnot_GetFormFieldType(&requests.FPDFAnnot_GetFormFieldType{FormHandle: formHandle, Annotation: annotation}); err == nil {
		field.Type = pdfFormFieldTypeName(resp.FormFieldType)
	}
	if resp, err := instance.FPDFAnnot_GetFormFieldValue(&requests.FPDFAnnot_GetFormFieldValue{FormHandle: formHandle, Annotation: annotation}); err == nil {
		field.Value = strings.TrimSpace(resp.FormFieldValue)
	}
	if resp, err := instance.FPDFAnnot_GetFormFieldExportValue(&requests.FPDFAnnot_GetFormFieldExportValue{FormHandle: formHandle, Annotation: annotation}); err == nil {
		field.ExportValue = strings.TrimSpace(resp.Value)
	}
	if resp, err := instance.FPDFAnnot_GetFormFieldFlags(&requests.FPDFAnnot_GetFormFieldFlags{FormHandle: formHandle, Annotation: annotation}); err == nil {
		field.ReadOnly = resp.Flags&enums.FPDF_FORMFLAG_READONLY != 0
		field.Required = resp.Flags&enums.FPDF_FORMFLAG_REQUIRED != 0
		field.NoExport = resp.Flags&enums.FPDF_FORMFLAG_NOEXPORT != 0
	}
	if resp, err := instance.FPDFAnnot_GetRect(&requests.FPDFAnnot_GetRect{Annotation: annotation}); err == nil {
		field.Rect = &FormFieldRect{
			Left:   float64(resp.Rect.Left),
			Top:    float64(resp.Rect.Top),
			Right:  float64(resp.Rect.Right),
			Bottom: float64(resp.Rect.Bottom),
		}
	}

	switch field.Type {
	case "checkbox", "radio":
		if resp, err := instance.FPDFAnnot_IsChecked(&requests.FPDFAnnot_IsChecked{FormHandle: formHandle, Annotation: annotation}); err == nil {
			field.Checked = resp.IsChecked
		}
	case "combo", "list":
		if resp, err := instance.FPDFAnnot_GetOptionCount(&requests.FPDFAnnot_GetOptionCount{FormHandle: formHandle, Annotation: annotation}); err == nil {
			field.Options = make([]FormFieldOption, 0, resp.OptionCount)
			for optionIndex := 0; optionIndex < resp.OptionCount; optionIndex++ {
				option := FormFieldOption{Index: optionIndex}
				if label, err := instance.FPDFAnnot_GetOptionLabel(&requests.FPDFAnnot_GetOptionLabel{FormHandle: formHandle, Annotation: annotation, Index: optionIndex}); err == nil {
					option.Label = strings.TrimSpace(label.OptionLabel)
				}
				if selected, err := instance.FPDFAnnot_IsOptionSelected(&requests.FPDFAnnot_IsOptionSelected{FormHandle: formHandle, Annotation: annotation, Index: optionIndex}); err == nil {
					option.Selected = selected.IsOptionSelected
				}
				field.Options = append(field.Options, option)
			}
		}
	}

	return field, true, warnings
}

func pdfFormTypeName(formType enums.FPDF_FORMTYPE) string {
	switch formType {
	case enums.FPDF_FORMTYPE_NONE:
		return "none"
	case enums.FPDF_FORMTYPE_ACRO_FORM:
		return "acro_form"
	case enums.FPDF_FORMTYPE_XFA_FULL:
		return "xfa_full"
	case enums.FPDF_FORMTYPE_XFA_FOREGROUND:
		return "xfa_foreground"
	default:
		return "unknown"
	}
}

func pdfFormFieldTypeName(fieldType enums.FPDF_FORMFIELD_TYPE) string {
	switch fieldType {
	case enums.FPDF_FORMFIELD_TYPE_PUSHBUTTON:
		return "button"
	case enums.FPDF_FORMFIELD_TYPE_CHECKBOX:
		return "checkbox"
	case enums.FPDF_FORMFIELD_TYPE_RADIOBUTTON:
		return "radio"
	case enums.FPDF_FORMFIELD_TYPE_COMBOBOX:
		return "combo"
	case enums.FPDF_FORMFIELD_TYPE_LISTBOX:
		return "list"
	case enums.FPDF_FORMFIELD_TYPE_TEXTFIELD:
		return "text"
	case enums.FPDF_FORMFIELD_TYPE_SIGNATURE:
		return "signature"
	case enums.FPDF_FORMFIELD_TYPE_XFA:
		return "xfa"
	case enums.FPDF_FORMFIELD_TYPE_XFA_CHECKBOX:
		return "xfa_checkbox"
	case enums.FPDF_FORMFIELD_TYPE_XFA_COMBOBOX:
		return "xfa_combo"
	case enums.FPDF_FORMFIELD_TYPE_XFA_IMAGEFIELD:
		return "xfa_image"
	case enums.FPDF_FORMFIELD_TYPE_XFA_LISTBOX:
		return "xfa_list"
	case enums.FPDF_FORMFIELD_TYPE_XFA_PUSHBUTTON:
		return "xfa_button"
	case enums.FPDF_FORMFIELD_TYPE_XFA_SIGNATURE:
		return "xfa_signature"
	case enums.FPDF_FORMFIELD_TYPE_XFA_TEXTFIELD:
		return "xfa_text"
	default:
		return "unknown"
	}
}

func compactFormWarnings(warnings []string) []string {
	if len(warnings) == 0 {
		return nil
	}
	out := make([]string, 0, len(warnings))
	seen := make(map[string]struct{}, len(warnings))
	for _, warning := range warnings {
		warning = strings.TrimSpace(warning)
		if warning == "" {
			continue
		}
		if _, ok := seen[warning]; ok {
			continue
		}
		seen[warning] = struct{}{}
		out = append(out, warning)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
