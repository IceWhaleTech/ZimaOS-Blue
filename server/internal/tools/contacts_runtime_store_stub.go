//go:build !darwin

package tools

func defaultContactsNativeStoreFactory(ContactsStore) ContactsStore {
	return nil
}
