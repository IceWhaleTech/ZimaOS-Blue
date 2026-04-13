package a11y

import (
	"fmt"
	"strings"

	"github.com/go-ole/go-ole"
)

func windowsVariantTextValue(
	value *ole.VARIANT,
	numberLookup func(uint32) string,
	stringValue func(*ole.VARIANT) string,
) string {
	if value == nil {
		return ""
	}
	switch value.VT {
	case ole.VT_I4:
		if numberLookup == nil {
			return ""
		}
		return strings.TrimSpace(numberLookup(uint32(value.Val)))
	case ole.VT_BSTR:
		if stringValue != nil {
			return strings.TrimSpace(stringValue(value))
		}
		return strings.TrimSpace(value.ToString())
	default:
		if raw := value.Value(); raw != nil {
			return strings.TrimSpace(fmt.Sprint(raw))
		}
		return ""
	}
}
