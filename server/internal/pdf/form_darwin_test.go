//go:build darwin

package pdf

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"go.uber.org/zap"
)

func TestServiceInspectFormReturnsTextFieldMetadataOnDarwin(t *testing.T) {
	path := filepath.Join(t.TempDir(), "form.pdf")
	if err := os.WriteFile(path, buildTextFieldPDF("full_name", "Full Name", "Alice"), 0o644); err != nil {
		t.Fatalf("write pdf fixture: %v", err)
	}

	svc := NewService(zap.NewNop(), nil)
	defer func() {
		_ = svc.Close()
	}()

	result, err := svc.InspectForm(context.Background(), path)
	if err != nil {
		t.Fatalf("InspectForm returned error: %v", err)
	}
	if result.FormType != "acro_form" {
		t.Fatalf("form_type = %q, want acro_form", result.FormType)
	}
	if result.FieldCount != 1 || len(result.Fields) != 1 {
		t.Fatalf("fields = %#v, want exactly 1 field", result.Fields)
	}
	if result.Document.Engine != darwinPDFKitEngineName {
		t.Fatalf("document.engine = %q, want %q", result.Document.Engine, darwinPDFKitEngineName)
	}

	field := result.Fields[0]
	if field.Name != "full_name" {
		t.Fatalf("name = %q, want full_name", field.Name)
	}
	if field.AlternateName != "Full Name" {
		t.Fatalf("alternate_name = %q, want Full Name", field.AlternateName)
	}
	if field.Type != "text" {
		t.Fatalf("type = %q, want text", field.Type)
	}
	if field.Value != "Alice" {
		t.Fatalf("value = %q, want Alice", field.Value)
	}
}

func TestServiceFillFormUpdatesTextFieldValueOnDarwin(t *testing.T) {
	path := filepath.Join(t.TempDir(), "form.pdf")
	if err := os.WriteFile(path, buildTextFieldPDF("full_name", "Full Name", "Alice"), 0o644); err != nil {
		t.Fatalf("write pdf fixture: %v", err)
	}

	svc := NewService(zap.NewNop(), nil)
	defer func() {
		_ = svc.Close()
	}()

	result, err := svc.FillForm(context.Background(), FillFormRequest{
		Path: path,
		Fields: map[string]string{
			"full_name": "Bob",
		},
	})
	if err != nil {
		t.Fatalf("FillForm returned error: %v", err)
	}
	if result.Document.Engine != darwinPDFKitEngineName {
		t.Fatalf("document.engine = %q, want %q", result.Document.Engine, darwinPDFKitEngineName)
	}
	if !reflect.DeepEqual(result.UpdatedFields, []string{"full_name"}) {
		t.Fatalf("updated_fields = %#v, want [full_name]", result.UpdatedFields)
	}
	if len(result.Bytes) == 0 || !bytes.HasPrefix(result.Bytes, []byte("%PDF-")) {
		t.Fatalf("unexpected filled pdf bytes")
	}

	outputPath := filepath.Join(t.TempDir(), "filled.pdf")
	if err := os.WriteFile(outputPath, result.Bytes, 0o644); err != nil {
		t.Fatalf("write filled pdf: %v", err)
	}

	inspected, err := svc.InspectForm(context.Background(), outputPath)
	if err != nil {
		t.Fatalf("InspectForm(filled) returned error: %v", err)
	}
	if inspected.FieldCount != 1 || len(inspected.Fields) != 1 {
		t.Fatalf("filled inspect fields = %#v, want 1 field", inspected.Fields)
	}
	if inspected.Fields[0].Value != "Bob" {
		t.Fatalf("filled field value = %q, want Bob", inspected.Fields[0].Value)
	}
}

func TestServiceFillFormUpdatesComboFieldSelectionOnDarwin(t *testing.T) {
	path := filepath.Join(t.TempDir(), "combo.pdf")
	if err := os.WriteFile(path, buildComboFieldPDF("favorite_color", "Favorite Color", "Red", []string{"Red", "Green", "Blue"}), 0o644); err != nil {
		t.Fatalf("write pdf fixture: %v", err)
	}

	svc := NewService(zap.NewNop(), nil)
	defer func() {
		_ = svc.Close()
	}()

	result, err := svc.FillForm(context.Background(), FillFormRequest{
		Path: path,
		Fields: map[string]string{
			"favorite_color": "Green",
		},
	})
	if err != nil {
		t.Fatalf("FillForm returned error: %v", err)
	}
	if !reflect.DeepEqual(result.UpdatedFields, []string{"favorite_color"}) {
		t.Fatalf("updated_fields = %#v, want [favorite_color]", result.UpdatedFields)
	}

	outputPath := filepath.Join(t.TempDir(), "combo-filled.pdf")
	if err := os.WriteFile(outputPath, result.Bytes, 0o644); err != nil {
		t.Fatalf("write filled pdf: %v", err)
	}

	inspected, err := svc.InspectForm(context.Background(), outputPath)
	if err != nil {
		t.Fatalf("InspectForm(filled) returned error: %v", err)
	}
	if inspected.FieldCount != 1 || len(inspected.Fields) != 1 {
		t.Fatalf("filled inspect fields = %#v, want 1 field", inspected.Fields)
	}
	field := inspected.Fields[0]
	if field.Type != "combo" {
		t.Fatalf("filled field type = %q, want combo", field.Type)
	}
	if field.Value != "Green" {
		t.Fatalf("filled combo value = %q, want Green", field.Value)
	}
	if len(field.Options) != 3 || !field.Options[1].Selected {
		t.Fatalf("filled combo options = %#v, want Green selected", field.Options)
	}
}

func TestServiceFillFormUpdatesCheckboxStateOnDarwin(t *testing.T) {
	path := filepath.Join(t.TempDir(), "checkbox.pdf")
	if err := os.WriteFile(path, buildCheckboxFieldPDF("subscribe", "Subscribe", false), 0o644); err != nil {
		t.Fatalf("write pdf fixture: %v", err)
	}

	svc := NewService(zap.NewNop(), nil)
	defer func() {
		_ = svc.Close()
	}()

	result, err := svc.FillForm(context.Background(), FillFormRequest{
		Path: path,
		Fields: map[string]string{
			"subscribe": "true",
		},
	})
	if err != nil {
		t.Fatalf("FillForm returned error: %v", err)
	}
	if !reflect.DeepEqual(result.UpdatedFields, []string{"subscribe"}) {
		t.Fatalf("updated_fields = %#v, want [subscribe]", result.UpdatedFields)
	}

	outputPath := filepath.Join(t.TempDir(), "checkbox-filled.pdf")
	if err := os.WriteFile(outputPath, result.Bytes, 0o644); err != nil {
		t.Fatalf("write filled pdf: %v", err)
	}

	inspected, err := svc.InspectForm(context.Background(), outputPath)
	if err != nil {
		t.Fatalf("InspectForm(filled) returned error: %v", err)
	}
	if inspected.FieldCount != 1 || len(inspected.Fields) != 1 {
		t.Fatalf("filled inspect fields = %#v, want 1 field", inspected.Fields)
	}
	field := inspected.Fields[0]
	if field.Type != "checkbox" {
		t.Fatalf("filled field type = %q, want checkbox", field.Type)
	}
	if !field.Checked {
		t.Fatalf("filled checkbox = %#v, want checked", field)
	}
}

func TestServiceFillFormRejectsInvalidCheckboxValueOnDarwin(t *testing.T) {
	path := filepath.Join(t.TempDir(), "checkbox.pdf")
	if err := os.WriteFile(path, buildCheckboxFieldPDF("subscribe", "Subscribe", false), 0o644); err != nil {
		t.Fatalf("write pdf fixture: %v", err)
	}

	svc := NewService(zap.NewNop(), nil)
	defer func() {
		_ = svc.Close()
	}()

	_, err := svc.FillForm(context.Background(), FillFormRequest{
		Path: path,
		Fields: map[string]string{
			"subscribe": "maybe",
		},
	})
	if err == nil {
		t.Fatal("expected invalid checkbox value fill to fail")
	}
	if !strings.Contains(err.Error(), "maybe") {
		t.Fatalf("error = %v, want invalid value in error", err)
	}
}

func TestServiceFillFormUpdatesRadioSelectionOnDarwin(t *testing.T) {
	path := filepath.Join(t.TempDir(), "radio.pdf")
	if err := os.WriteFile(path, buildRadioFieldPDF("contact_method", "Contact Method", "Email", []string{"Email", "Phone"}), 0o644); err != nil {
		t.Fatalf("write pdf fixture: %v", err)
	}

	svc := NewService(zap.NewNop(), nil)
	defer func() {
		_ = svc.Close()
	}()

	result, err := svc.FillForm(context.Background(), FillFormRequest{
		Path: path,
		Fields: map[string]string{
			"contact_method": "Phone",
		},
	})
	if err != nil {
		t.Fatalf("FillForm returned error: %v", err)
	}
	if !reflect.DeepEqual(result.UpdatedFields, []string{"contact_method"}) {
		t.Fatalf("updated_fields = %#v, want [contact_method]", result.UpdatedFields)
	}

	outputPath := filepath.Join(t.TempDir(), "radio-filled.pdf")
	if err := os.WriteFile(outputPath, result.Bytes, 0o644); err != nil {
		t.Fatalf("write filled pdf: %v", err)
	}

	inspected, err := svc.InspectForm(context.Background(), outputPath)
	if err != nil {
		t.Fatalf("InspectForm(filled) returned error: %v", err)
	}
	if inspected.FieldCount != 2 || len(inspected.Fields) != 2 {
		t.Fatalf("filled inspect fields = %#v, want 2 radio widgets", inspected.Fields)
	}
	var emailChecked, phoneChecked bool
	for _, field := range inspected.Fields {
		if field.Type != "radio" {
			t.Fatalf("filled field type = %q, want radio", field.Type)
		}
		switch field.ExportValue {
		case "Email":
			emailChecked = field.Checked
		case "Phone":
			phoneChecked = field.Checked
		}
	}
	if emailChecked || !phoneChecked {
		t.Fatalf("radio checked states = email:%v phone:%v, want email:false phone:true", emailChecked, phoneChecked)
	}
}

func TestServiceFillFormRejectsUnknownRadioExportValueOnDarwin(t *testing.T) {
	path := filepath.Join(t.TempDir(), "radio.pdf")
	if err := os.WriteFile(path, buildRadioFieldPDF("contact_method", "Contact Method", "Email", []string{"Email", "Phone"}), 0o644); err != nil {
		t.Fatalf("write pdf fixture: %v", err)
	}

	svc := NewService(zap.NewNop(), nil)
	defer func() {
		_ = svc.Close()
	}()

	_, err := svc.FillForm(context.Background(), FillFormRequest{
		Path: path,
		Fields: map[string]string{
			"contact_method": "SMS",
		},
	})
	if err == nil {
		t.Fatal("expected unknown radio export fill to fail")
	}
	if !strings.Contains(err.Error(), "SMS") {
		t.Fatalf("error = %v, want export value in error", err)
	}
}
