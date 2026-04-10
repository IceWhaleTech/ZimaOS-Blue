//go:build darwin

package tools

func defaultContactsNativeStoreFactory(fallback ContactsStore) ContactsStore {
	return newDarwinNativeContactsStoreOrNil(fallback)
}
