package tools

import "sync"

var (
	contactsNativeStoreFactoryMu sync.RWMutex
	contactsNativeStoreFactory   = defaultContactsNativeStoreFactory
)

// PreferredContactsStore chooses the runtime-backed contacts store when one is
// available and otherwise falls back to the provided backend.
func PreferredContactsStore(fallback ContactsStore) ContactsStore {
	if fallback == nil {
		return nil
	}
	contactsNativeStoreFactoryMu.RLock()
	factory := contactsNativeStoreFactory
	contactsNativeStoreFactoryMu.RUnlock()
	if factory == nil {
		return fallback
	}
	if native := factory(fallback); native != nil {
		return native
	}
	return fallback
}

func setContactsNativeStoreFactoryForTest(factory func(ContactsStore) ContactsStore) func() {
	contactsNativeStoreFactoryMu.Lock()
	prev := contactsNativeStoreFactory
	contactsNativeStoreFactory = factory
	contactsNativeStoreFactoryMu.Unlock()
	return func() {
		contactsNativeStoreFactoryMu.Lock()
		contactsNativeStoreFactory = prev
		contactsNativeStoreFactoryMu.Unlock()
	}
}
