package a11y

import (
	"testing"

	"github.com/go-ole/go-ole"
)

func TestWindowsVariantInt32Value_ConvertsCommonIntegerShapes(t *testing.T) {
	tests := []struct {
		name  string
		value *ole.VARIANT
		want  int32
		ok    bool
	}{
		{name: "int32", value: variantPtr(ole.NewVariant(ole.VT_I4, int64(7))), want: 7, ok: true},
		{name: "int64", value: variantPtr(ole.NewVariant(ole.VT_I8, int64(11))), want: 11, ok: true},
		{name: "nil", value: nil, want: 0, ok: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := windowsVariantInt32Value(tc.value)
			if got != tc.want || ok != tc.ok {
				t.Fatalf("windowsVariantInt32Value() = (%d,%v), want (%d,%v)", got, ok, tc.want, tc.ok)
			}
		})
	}
}

func TestWindowsVariantInt32Value_ReturnsFalseForNonIntegerValue(t *testing.T) {
	value := &ole.VARIANT{VT: ole.VT_BSTR}
	if got, ok := windowsVariantInt32Value(value); got != 0 || ok {
		t.Fatalf("windowsVariantInt32Value() = (%d,%v), want (0,false)", got, ok)
	}
}

func variantPtr(value ole.VARIANT) *ole.VARIANT {
	return &value
}
