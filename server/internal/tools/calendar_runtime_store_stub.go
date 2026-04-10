//go:build !darwin

package tools

func defaultCalendarNativeStoreFactory(_ CalendarStore) CalendarStore {
	return nil
}
