package tools

import "sync"

var (
	calendarNativeStoreFactoryMu sync.RWMutex
	calendarNativeStoreFactory   = defaultCalendarNativeStoreFactory
)

// PreferredCalendarStore chooses the runtime-backed calendar store when one is
// available and otherwise falls back to the provided local store.
func PreferredCalendarStore(local CalendarStore) CalendarStore {
	if local == nil {
		return nil
	}
	calendarNativeStoreFactoryMu.RLock()
	factory := calendarNativeStoreFactory
	calendarNativeStoreFactoryMu.RUnlock()
	if factory == nil {
		return local
	}
	if native := factory(local); native != nil {
		return native
	}
	return local
}

func setCalendarNativeStoreFactoryForTest(factory func(CalendarStore) CalendarStore) func() {
	calendarNativeStoreFactoryMu.Lock()
	prev := calendarNativeStoreFactory
	calendarNativeStoreFactory = factory
	calendarNativeStoreFactoryMu.Unlock()
	return func() {
		calendarNativeStoreFactoryMu.Lock()
		calendarNativeStoreFactory = prev
		calendarNativeStoreFactoryMu.Unlock()
	}
}
