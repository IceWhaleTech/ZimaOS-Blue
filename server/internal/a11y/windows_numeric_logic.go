package a11y

import "github.com/go-ole/go-ole"

func windowsVariantInt32Value(value *ole.VARIANT) (int32, bool) {
	if value == nil {
		return 0, false
	}
	switch raw := value.Value().(type) {
	case int32:
		return raw, true
	case int:
		return int32(raw), true
	case int64:
		return int32(raw), true
	default:
		return 0, false
	}
}
