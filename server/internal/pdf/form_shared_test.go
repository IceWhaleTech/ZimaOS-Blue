package pdf

import "testing"

func TestClassifyNativePDFWidgetTypeAcceptsPDFNames(t *testing.T) {
	for _, test := range []struct {
		raw, want string
		list      bool
	}{
		{"Tx", "text", false},
		{"Ch", "combo", false},
		{"Ch", "list", true},
		{"Btn", "button", false},
		{"Sig", "signature", false},
		{"/Tx", "text", false},
		{"/Ch", "combo", false},
		{"/Btn", "button", false},
		{"/Sig", "signature", false},
	} {
		t.Run(test.raw+test.want, func(t *testing.T) {
			if got := classifyNativePDFWidgetType(test.raw, test.list); got != test.want {
				t.Fatalf("type = %q, want %q", got, test.want)
			}
		})
	}
}
