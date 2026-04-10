//go:build darwin

package tools

func defaultCalendarNativeStoreFactory(local CalendarStore) CalendarStore {
	return newDarwinNativeCalendarStoreOrNil(local)
}
