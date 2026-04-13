package a11y

import (
	"testing"

	"github.com/go-ole/go-ole"
)

func TestWindowsVariantTextValue_UsesNumericLookup(t *testing.T) {
	value := ole.NewVariant(ole.VT_I4, int64(7))

	got := windowsVariantTextValue(&value, func(raw uint32) string {
		if raw != 7 {
			t.Fatalf("raw = %d, want 7", raw)
		}
		return "  push button  "
	}, nil)
	if got != "push button" {
		t.Fatalf("windowsVariantTextValue() = %q, want push button", got)
	}
}

func TestWindowsVariantTextValue_UsesStringConverterForBSTR(t *testing.T) {
	value := &ole.VARIANT{VT: ole.VT_BSTR}

	got := windowsVariantTextValue(value, nil, func(v *ole.VARIANT) string {
		if v != value {
			t.Fatal("unexpected variant pointer")
		}
		return "  menu item  "
	})
	if got != "menu item" {
		t.Fatalf("windowsVariantTextValue() = %q, want menu item", got)
	}
}

func TestWindowsVariantTextValue_FallsBackToRawValueString(t *testing.T) {
	value := ole.NewVariant(ole.VT_I8, int64(123))

	got := windowsVariantTextValue(&value, nil, nil)
	if got != "123" {
		t.Fatalf("windowsVariantTextValue() = %q, want 123", got)
	}
}

func TestWindowsVariantTextValue_ReturnsEmptyForNil(t *testing.T) {
	if got := windowsVariantTextValue(nil, nil, nil); got != "" {
		t.Fatalf("windowsVariantTextValue(nil) = %q, want empty", got)
	}
}
