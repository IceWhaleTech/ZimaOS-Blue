// +build !windows

package speech

import "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/stt"

func NewWindowsNativeASR() stt.Provider {
	return nil
}
