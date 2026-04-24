//go:build darwin

package pdf

import (
	"context"
	"fmt"
	"runtime"
	"strconv"
	"strings"

	"github.com/ebitengine/purego/objc"
)

const (
	darwinPDFNativeFormFillUnsupportedMessage = "macOS native PDF form filling currently supports text fields, combo boxes, list boxes, checkboxes, and radio buttons only"

	darwinPDFAnnotationKeyWidgetFieldType   = "widgetFieldType"
	darwinPDFAnnotationKeyWidgetControlType = "widgetControlType"
	darwinPDFAnnotationKeyWidgetTextLabel   = "widgetTextLabelUI"
	darwinPDFAnnotationKeyWidgetFieldFlags  = "widgetFieldFlags"
	darwinPDFAnnotationKeyWidgetValue       = "widgetValue"
	darwinPDFAnnotationKeyWidgetOptions     = "widgetOptions"

	darwinPDFFormFlagReadOnly = 1 << 0
	darwinPDFFormFlagRequired = 1 << 1
	darwinPDFFormFlagNoExport = 1 << 2

	darwinPDFButtonWidgetStateOff int64 = 0
	darwinPDFButtonWidgetStateOn  int64 = 1
)

type darwinPDFFormWidget struct {
	PageIndex         int
	PageNumber        int
	Annotation        objc.ID
	Field             FormField
	RawType           string
	ButtonControlType string
	ButtonState       int64
	ButtonCaption     string
}

func pdfFormSupportAvailable() bool {
	return true
}

func (s *Service) InspectForm(ctx context.Context, path string) (FormInspectResult, error) {
	if err := ctx.Err(); err != nil {
		return FormInspectResult{}, err
	}
	resolvedPath, stat, err := resolvePath(path)
	if err != nil {
		return FormInspectResult{}, err
	}

	var result FormInspectResult
	err = withDarwinPDFFormDocument(ctx, resolvedPath, func(document objc.ID, pageCount int, metadata map[string]string) error {
		widgets, warnings, sawWidget, err := collectDarwinPDFFormWidgets(ctx, document, pageCount)
		if err != nil {
			return err
		}
		result.Document = darwinPDFKitDocumentInfo(resolvedPath, stat, pageCount, metadata)
		if sawWidget {
			result.FormType = "acro_form"
		} else {
			result.FormType = "none"
		}
		result.Fields = make([]FormField, 0, len(widgets))
		for _, widget := range widgets {
			result.Fields = append(result.Fields, widget.Field)
		}
		result.FieldCount = len(result.Fields)
		result.Warnings = compactFormWarnings(warnings)
		return nil
	})
	if err != nil {
		return FormInspectResult{}, fmt.Errorf("inspect pdf form with macOS PDFKit: %w", err)
	}
	return result, nil
}

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

	var result FillFormResult
	err = withDarwinPDFFormDocument(ctx, resolvedPath, func(document objc.ID, pageCount int, metadata map[string]string) error {
		widgets, warnings, sawWidget, err := collectDarwinPDFFormWidgets(ctx, document, pageCount)
		if err != nil {
			return err
		}
		if !sawWidget {
			return fmt.Errorf("pdf does not contain an interactive form")
		}

		lookup := groupDarwinPDFFormWidgets(widgets)
		updatedFields := make([]string, 0, len(req.Fields))
		for _, fieldName := range sortedPDFFieldNames(req.Fields) {
			candidates, ok := lookup[fieldName]
			if !ok || len(candidates) == 0 {
				return fmt.Errorf("requested pdf form field not found: %s", fieldName)
			}
			target := candidates[0]
			if target.Field.Type != "radio" && target.Field.ReadOnly {
				return fmt.Errorf("pdf form field %q is read-only", fieldName)
			}
			if !supportsNativePDFFormFill(target.Field.Type) {
				return fmt.Errorf("%s; field %q has type %q", darwinPDFNativeFormFillUnsupportedMessage, fieldName, target.Field.Type)
			}

			switch target.Field.Type {
			case "radio":
				fields := make([]FormField, 0, len(candidates))
				for _, candidate := range candidates {
					fields = append(fields, candidate.Field)
				}
				selectedIndex, err := resolvePDFRadioFieldIndex(fields, req.Fields[fieldName])
				if err != nil {
					return fmt.Errorf("set radio selection for pdf form field %q: %w", fieldName, err)
				}
				if err := ensurePDFRadioGroupWritable(fields); err != nil {
					return fmt.Errorf("set radio selection for pdf form field %q: %w", fieldName, err)
				}
				for index, candidate := range candidates {
					checked := index == selectedIndex
					if err := setDarwinPDFButtonState(candidate.Annotation, checked); err != nil {
						return fmt.Errorf("set radio selection for pdf form field %q: %w", fieldName, err)
					}
				}
			default:
				if err := setDarwinPDFFormFieldValue(target.Annotation, &target.Field, req.Fields[fieldName]); err != nil {
					return fmt.Errorf("set pdf form field %q: %w", fieldName, err)
				}
			}
			updatedFields = append(updatedFields, fieldName)
		}

		data := document.Send(darwinPDFSelDataRepresentation)
		bytes := darwinPDFDataBytes(data)
		if len(bytes) == 0 {
			return fmt.Errorf("save filled pdf: empty output")
		}

		result = FillFormResult{
			Document:      darwinPDFKitDocumentInfo(resolvedPath, stat, pageCount, metadata),
			Bytes:         bytes,
			UpdatedFields: updatedFields,
			Warnings:      compactFormWarnings(warnings),
		}
		return nil
	})
	if err != nil {
		return FillFormResult{}, fmt.Errorf("fill pdf form with macOS PDFKit: %w", err)
	}
	return result, nil
}

func withDarwinPDFFormDocument(ctx context.Context, path string, fn func(document objc.ID, pageCount int, metadata map[string]string) error) error {
	if err := initDarwinPDFKitSelectors(); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	pool := newDarwinPDFAutoreleasePool()
	if pool != 0 {
		defer releaseDarwinPDFObject(pool)
	}

	urlClass := objc.ID(objc.GetClass("NSURL"))
	documentClass := objc.ID(objc.GetClass("PDFDocument"))
	if urlClass == 0 || documentClass == 0 {
		return errDarwinPDFKitUnavailable
	}

	nsPath := darwinPDFNSString(path)
	if nsPath == 0 {
		return fmt.Errorf("create path string for PDFKit")
	}
	url := urlClass.Send(darwinPDFSelFileURLWithPath, nsPath)
	if url == 0 {
		return fmt.Errorf("create file URL for PDFKit")
	}

	document := documentClass.Send(darwinPDFSelAlloc).Send(darwinPDFSelInitWithURL, url)
	if document == 0 {
		return fmt.Errorf("open PDF document")
	}
	defer releaseDarwinPDFObject(document)

	pageCount := int(objc.Send[uint64](document, darwinPDFSelPageCount))
	return fn(document, pageCount, darwinPDFDocumentAttributes(document))
}

func collectDarwinPDFFormWidgets(ctx context.Context, document objc.ID, pageCount int) ([]darwinPDFFormWidget, []string, bool, error) {
	widgets := make([]darwinPDFFormWidget, 0, 8)
	warnings := make([]string, 0, 4)
	sawWidget := false

	for pageIndex := 0; pageIndex < pageCount; pageIndex++ {
		if err := ctx.Err(); err != nil {
			return nil, warnings, sawWidget, err
		}
		page := document.Send(darwinPDFSelPageAtIndex, uintptr(pageIndex))
		if page == 0 {
			warnings = append(warnings, fmt.Sprintf("page %d could not be loaded for native form inspection", pageIndex+1))
			continue
		}
		annotations := page.Send(darwinPDFSelAnnotations)
		if annotations == 0 {
			continue
		}
		count := int(objc.Send[uint64](annotations, darwinPDFSelCount))
		for annotIndex := 0; annotIndex < count; annotIndex++ {
			annotation := annotations.Send(darwinPDFSelObjectAtIndex, uintptr(annotIndex))
			widget, ok, fieldWarnings := inspectDarwinPDFFormAnnotation(annotation, pageIndex+1)
			if len(fieldWarnings) > 0 {
				warnings = append(warnings, fieldWarnings...)
			}
			if !ok {
				continue
			}
			sawWidget = true
			widget.PageIndex = pageIndex
			widget.PageNumber = pageIndex + 1
			widget.Annotation = annotation
			widgets = append(widgets, widget)
		}
	}

	warnings = append(warnings, normalizeDarwinPDFButtonWidgets(widgets)...)
	return widgets, compactFormWarnings(warnings), sawWidget, nil
}

func inspectDarwinPDFFormAnnotation(annotation objc.ID, pageNumber int) (darwinPDFFormWidget, bool, []string) {
	if annotation == 0 {
		return darwinPDFFormWidget{}, false, nil
	}

	rawType := darwinPDFAnnotationStringValue(annotation, darwinPDFAnnotationKeyWidgetFieldType)
	if rawType == "" {
		return darwinPDFFormWidget{}, false, nil
	}

	isListChoice := darwinPDFBoolProperty(annotation, darwinPDFSelIsListChoice)
	field := FormField{
		PageNumber: pageNumber,
		Type:       classifyNativePDFWidgetType(rawType, isListChoice),
		Name:       strings.TrimSpace(darwinPDFStringProperty(annotation, darwinPDFSelFieldName)),
		Value:      strings.TrimSpace(darwinPDFStringProperty(annotation, darwinPDFSelWidgetStringValue)),
	}
	field.AlternateName = strings.TrimSpace(darwinPDFAnnotationStringValue(annotation, darwinPDFAnnotationKeyWidgetTextLabel))
	if field.Value == "" {
		field.Value = strings.TrimSpace(darwinPDFAnnotationStringValue(annotation, darwinPDFAnnotationKeyWidgetValue))
	}

	flags := darwinPDFAnnotationIntValue(annotation, darwinPDFAnnotationKeyWidgetFieldFlags)
	field.ReadOnly = darwinPDFBoolProperty(annotation, darwinPDFSelIsReadOnly) || flags&darwinPDFFormFlagReadOnly != 0
	field.Required = flags&darwinPDFFormFlagRequired != 0
	field.NoExport = flags&darwinPDFFormFlagNoExport != 0

	if rect, ok := darwinPDFRectProperty(annotation, darwinPDFSelBounds); ok {
		field.Rect = &FormFieldRect{
			Left:   rect.Origin.X,
			Top:    rect.Origin.Y + rect.Size.Height,
			Right:  rect.Origin.X + rect.Size.Width,
			Bottom: rect.Origin.Y,
		}
	}

	widget := darwinPDFFormWidget{
		Field:   field,
		RawType: strings.ToLower(strings.TrimSpace(rawType)),
	}

	warnings := make([]string, 0, 2)
	switch field.Type {
	case "combo", "list":
		options := darwinPDFStringArray(annotation.Send(darwinPDFSelChoices))
		if len(options) == 0 {
			options = darwinPDFStringArray(darwinPDFAnnotationValue(annotation, darwinPDFAnnotationKeyWidgetOptions))
		}
		selectedValues := darwinPDFStringSet(annotation.Send(darwinPDFSelValues))
		if field.Value != "" {
			selectedValues[field.Value] = struct{}{}
		}
		field.Options = make([]FormFieldOption, 0, len(options))
		for index, label := range options {
			option := FormFieldOption{
				Index:    index,
				Label:    label,
				Selected: false,
			}
			if _, ok := selectedValues[label]; ok {
				option.Selected = true
			}
			if !option.Selected && strings.EqualFold(label, field.Value) {
				option.Selected = true
			}
			field.Options = append(field.Options, option)
		}
		widget.Field = field
	case "button":
		widget.ButtonControlType = strings.TrimSpace(darwinPDFAnnotationStringValue(annotation, darwinPDFAnnotationKeyWidgetControlType))
		widget.ButtonState = darwinPDFButtonState(annotation)
		widget.ButtonCaption = strings.TrimSpace(darwinPDFStringProperty(annotation, darwinPDFSelCaption))
		widget.Field.ExportValue = strings.TrimSpace(darwinPDFStringProperty(annotation, darwinPDFSelButtonWidgetStateString))
		widget.Field.Checked = widget.ButtonState != darwinPDFButtonWidgetStateOff
		if !widget.Field.Checked && widget.Field.ExportValue != "" && strings.EqualFold(widget.Field.Value, widget.Field.ExportValue) {
			widget.Field.Checked = true
		}
	case "signature", "unknown":
		warnings = append(warnings, fmt.Sprintf("page %d widget %q has unsupported native widget type %q", pageNumber, darwinPDFFieldDisplayName(field), rawType))
	}

	return widget, true, warnings
}

func setDarwinPDFFormFieldValue(annotation objc.ID, field *FormField, requested string) error {
	if annotation == 0 || field == nil {
		return fmt.Errorf("missing native widget reference")
	}
	switch field.Type {
	case "text":
		value := darwinPDFNSString(requested)
		if value == 0 {
			return fmt.Errorf("create native text value")
		}
		annotation.Send(darwinPDFSelSetWidgetStringValue, value)
		field.Value = requested
		return nil
	case "combo", "list":
		optionIndex, err := resolvePDFChoiceOptionIndex(*field, requested)
		if err != nil {
			return err
		}
		selected := darwinPDFOptionLabel(*field, optionIndex, requested)
		value := darwinPDFNSString(selected)
		if value == 0 {
			return fmt.Errorf("create native choice value")
		}
		annotation.Send(darwinPDFSelSetWidgetStringValue, value)
		field.Value = selected
		for idx := range field.Options {
			field.Options[idx].Selected = field.Options[idx].Index == optionIndex
		}
		return nil
	case "checkbox":
		checked, err := resolvePDFCheckboxState(*field, requested)
		if err != nil {
			return err
		}
		if err := setDarwinPDFButtonState(annotation, checked); err != nil {
			return err
		}
		field.Checked = checked
		if checked {
			if field.ExportValue != "" {
				field.Value = field.ExportValue
			}
		} else {
			field.Value = ""
		}
		return nil
	default:
		return fmt.Errorf("%s", darwinPDFNativeFormFillUnsupportedMessage)
	}
}

func normalizeDarwinPDFButtonWidgets(widgets []darwinPDFFormWidget) []string {
	if len(widgets) == 0 {
		return nil
	}

	groups := make(map[string][]int)
	for index := range widgets {
		widget := &widgets[index]
		if widget.Field.Type != "button" {
			continue
		}
		groups[darwinPDFButtonGroupKey(*widget)] = append(groups[darwinPDFButtonGroupKey(*widget)], index)
	}

	warnings := make([]string, 0, len(groups))
	for _, indexes := range groups {
		groupValue := darwinPDFButtonGroupValue(widgets, indexes)
		groupSize := len(indexes)
		for _, index := range indexes {
			widget := &widgets[index]
			widget.Field.Type = classifyNativePDFButtonFieldType(widget.ButtonControlType, widget.Field.ExportValue, groupSize)
			switch widget.Field.Type {
			case "radio":
				widget.Field.Checked = widget.Field.ExportValue != "" && strings.EqualFold(widget.Field.ExportValue, groupValue)
			case "checkbox":
				if widget.Field.ExportValue == "" {
					widget.Field.ExportValue = "Yes"
				}
				if !widget.Field.Checked && widget.Field.Value != "" && strings.EqualFold(widget.Field.Value, widget.Field.ExportValue) {
					widget.Field.Checked = true
				}
			default:
				warnings = append(warnings, fmt.Sprintf("page %d widget %q has native type %q, but macOS native fill currently supports text, choice, checkbox, and radio widgets only", widget.Field.PageNumber, darwinPDFFieldDisplayName(widget.Field), widget.Field.Type))
			}
		}
	}

	return warnings
}

func setDarwinPDFButtonState(annotation objc.ID, checked bool) error {
	if annotation == 0 {
		return fmt.Errorf("missing native button widget reference")
	}
	state := darwinPDFButtonWidgetStateOff
	if checked {
		state = darwinPDFButtonWidgetStateOn
	}
	if !darwinPDFRespondsToSelector(annotation, darwinPDFSelSetButtonWidgetState) {
		return fmt.Errorf("native button widget state setter unavailable")
	}
	annotation.Send(darwinPDFSelSetButtonWidgetState, state)
	return nil
}

func darwinPDFButtonState(annotation objc.ID) int64 {
	if !darwinPDFRespondsToSelector(annotation, darwinPDFSelButtonWidgetState) {
		return darwinPDFButtonWidgetStateOff
	}
	return objc.Send[int64](annotation, darwinPDFSelButtonWidgetState)
}

func darwinPDFButtonGroupKey(widget darwinPDFFormWidget) string {
	if name := strings.TrimSpace(widget.Field.Name); name != "" {
		return name
	}
	if alt := strings.TrimSpace(widget.Field.AlternateName); alt != "" {
		return alt
	}
	return fmt.Sprintf("page:%d:export:%s", widget.Field.PageNumber, strings.TrimSpace(widget.Field.ExportValue))
}

func darwinPDFButtonGroupValue(widgets []darwinPDFFormWidget, indexes []int) string {
	for _, index := range indexes {
		value := strings.TrimSpace(widgets[index].Field.Value)
		if value != "" {
			return value
		}
	}
	for _, index := range indexes {
		if widgets[index].Field.Checked {
			value := strings.TrimSpace(widgets[index].Field.ExportValue)
			if value != "" {
				return value
			}
		}
	}
	return ""
}

func groupDarwinPDFFormWidgets(widgets []darwinPDFFormWidget) map[string][]*darwinPDFFormWidget {
	groups := make(map[string][]*darwinPDFFormWidget, len(widgets))
	for idx := range widgets {
		widget := &widgets[idx]
		if name := strings.TrimSpace(widget.Field.Name); name != "" {
			groups[name] = append(groups[name], widget)
		}
		if alt := strings.TrimSpace(widget.Field.AlternateName); alt != "" && alt != strings.TrimSpace(widget.Field.Name) {
			groups[alt] = append(groups[alt], widget)
		}
	}
	return groups
}

func darwinPDFStringProperty(object objc.ID, sel objc.SEL) string {
	if !darwinPDFRespondsToSelector(object, sel) {
		return ""
	}
	return darwinPDFObjectString(object.Send(sel))
}

func darwinPDFBoolProperty(object objc.ID, sel objc.SEL) bool {
	if !darwinPDFRespondsToSelector(object, sel) {
		return false
	}
	return objc.Send[bool](object, sel)
}

func darwinPDFRectProperty(object objc.ID, sel objc.SEL) (darwinPDFRect, bool) {
	if !darwinPDFRespondsToSelector(object, sel) {
		return darwinPDFRect{}, false
	}
	return objc.Send[darwinPDFRect](object, sel), true
}

func darwinPDFRespondsToSelector(object objc.ID, sel objc.SEL) bool {
	if object == 0 || sel == 0 || darwinPDFSelRespondsToSelector == 0 {
		return false
	}
	return objc.Send[bool](object, darwinPDFSelRespondsToSelector, sel)
}

func darwinPDFAnnotationValue(annotation objc.ID, key string) objc.ID {
	if annotation == 0 || key == "" || !darwinPDFRespondsToSelector(annotation, darwinPDFSelValueForAnnotationKey) {
		return 0
	}
	nsKey := darwinPDFNSString(key)
	if nsKey == 0 {
		return 0
	}
	return annotation.Send(darwinPDFSelValueForAnnotationKey, nsKey)
}

func darwinPDFAnnotationStringValue(annotation objc.ID, key string) string {
	return strings.TrimSpace(darwinPDFObjectString(darwinPDFAnnotationValue(annotation, key)))
}

func darwinPDFAnnotationIntValue(annotation objc.ID, key string) int {
	value := strings.TrimSpace(darwinPDFObjectString(darwinPDFAnnotationValue(annotation, key)))
	if value == "" {
		return 0
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0
	}
	return parsed
}

func darwinPDFStringArray(array objc.ID) []string {
	if array == 0 {
		return nil
	}
	count := int(objc.Send[uint64](array, darwinPDFSelCount))
	if count <= 0 {
		return nil
	}
	values := make([]string, 0, count)
	for index := 0; index < count; index++ {
		value := strings.TrimSpace(darwinPDFObjectString(array.Send(darwinPDFSelObjectAtIndex, uintptr(index))))
		if value == "" {
			continue
		}
		values = append(values, value)
	}
	if len(values) == 0 {
		return nil
	}
	return values
}

func darwinPDFStringSet(array objc.ID) map[string]struct{} {
	values := darwinPDFStringArray(array)
	if len(values) == 0 {
		return make(map[string]struct{})
	}
	out := make(map[string]struct{}, len(values))
	for _, value := range values {
		out[value] = struct{}{}
	}
	return out
}

func darwinPDFOptionLabel(field FormField, index int, fallback string) string {
	for _, option := range field.Options {
		if option.Index != index {
			continue
		}
		if label := strings.TrimSpace(option.Label); label != "" {
			return label
		}
		break
	}
	return strings.TrimSpace(fallback)
}

func darwinPDFFieldDisplayName(field FormField) string {
	if name := strings.TrimSpace(field.Name); name != "" {
		return name
	}
	if alt := strings.TrimSpace(field.AlternateName); alt != "" {
		return alt
	}
	return "unnamed"
}
