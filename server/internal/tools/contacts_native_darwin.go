//go:build darwin

package tools

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"
)

const (
	darwinContactsEntityTypeContacts = 0

	darwinContactsAuthorizationNotDetermined = 0
	darwinContactsAuthorizationRestricted    = 1
	darwinContactsAuthorizationDenied        = 2
	darwinContactsAuthorizationAuthorized    = 3
	darwinContactsAuthorizationLimited       = 4
)

const darwinContactsAuthorizationTimeout = 2 * time.Minute

var (
	darwinContactsOnce sync.Once
	darwinContactsErr  error

	darwinContactsSelAlloc                                   objc.SEL
	darwinContactsSelInit                                    objc.SEL
	darwinContactsSelRelease                                 objc.SEL
	darwinContactsSelStringWithUTF8                          objc.SEL
	darwinContactsSelUTF8String                              objc.SEL
	darwinContactsSelAuthorizationStatusForEntityType        objc.SEL
	darwinContactsSelRequestAccessForEntityTypeCompletion    objc.SEL
	darwinContactsSelEnumerateContactsWithFetchRequestError  objc.SEL
	darwinContactsSelUnifiedContactWithIdentifierKeysToFetch objc.SEL
	darwinContactsSelExecuteSaveRequestError                 objc.SEL
	darwinContactsSelInitWithKeysToFetch                     objc.SEL
	darwinContactsSelAddObject                               objc.SEL
	darwinContactsSelCount                                   objc.SEL
	darwinContactsSelObjectAtIndex                           objc.SEL
	darwinContactsSelIdentifier                              objc.SEL
	darwinContactsSelGivenName                               objc.SEL
	darwinContactsSelFamilyName                              objc.SEL
	darwinContactsSelOrganizationName                        objc.SEL
	darwinContactsSelJobTitle                                objc.SEL
	darwinContactsSelPhoneNumbers                            objc.SEL
	darwinContactsSelEmailAddresses                          objc.SEL
	darwinContactsSelPostalAddresses                         objc.SEL
	darwinContactsSelBirthday                                objc.SEL
	darwinContactsSelValue                                   objc.SEL
	darwinContactsSelStringValue                             objc.SEL
	darwinContactsSelStreet                                  objc.SEL
	darwinContactsSelYear                                    objc.SEL
	darwinContactsSelMonth                                   objc.SEL
	darwinContactsSelDay                                     objc.SEL
	darwinContactsSelSetGivenName                            objc.SEL
	darwinContactsSelSetFamilyName                           objc.SEL
	darwinContactsSelSetOrganizationName                     objc.SEL
	darwinContactsSelSetJobTitle                             objc.SEL
	darwinContactsSelSetPhoneNumbers                         objc.SEL
	darwinContactsSelSetEmailAddresses                       objc.SEL
	darwinContactsSelSetPostalAddresses                      objc.SEL
	darwinContactsSelSetBirthday                             objc.SEL
	darwinContactsSelSetStreet                               objc.SEL
	darwinContactsSelSetYear                                 objc.SEL
	darwinContactsSelSetMonth                                objc.SEL
	darwinContactsSelSetDay                                  objc.SEL
	darwinContactsSelPhoneNumberWithStringValue              objc.SEL
	darwinContactsSelLabeledValueWithLabelValue              objc.SEL
	darwinContactsSelAddContactToContainerWithIdentifier     objc.SEL
	darwinContactsSelLocalizedDescription                    objc.SEL

	darwinContactsHasUsageDescriptionFunc = func() bool {
		return darwinContactsHasUsageDescription("NSContactsUsageDescription")
	}
	darwinContactsAuthorizationStatusFunc = darwinContactsAuthorizationStatus
	darwinContactsRequestAccessFunc       = darwinContactsRequestAccess
)

type darwinNativeContactsStore struct{}

func newDarwinNativeContactsStoreOrNil(_ ContactsStore) ContactsStore {
	if err := initDarwinNativeContacts(); err != nil {
		return nil
	}
	if objc.GetClass("CNContactStore") == 0 ||
		objc.GetClass("CNContactFetchRequest") == 0 ||
		objc.GetClass("CNMutableContact") == 0 ||
		objc.GetClass("CNPhoneNumber") == 0 ||
		objc.GetClass("CNLabeledValue") == 0 {
		return nil
	}
	return &darwinNativeContactsStore{}
}

func (s *darwinNativeContactsStore) List(ctx context.Context, _ string, opts ContactsQueryOptions) ([]ContactRecord, error) {
	if err := darwinContactsEnsureAccess(); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	pool := newDarwinContactsAutoreleasePool()
	defer releaseDarwinContactsObject(pool)

	store, err := newDarwinContactsStore()
	if err != nil {
		return nil, err
	}
	defer releaseDarwinContactsObject(store)

	keys := darwinContactsKeysArray()
	if keys == 0 {
		return nil, fmt.Errorf("contacts native keys unavailable")
	}
	defer releaseDarwinContactsObject(keys)

	return darwinContactsEnumerate(store, keys, ctx, opts)
}

func (s *darwinNativeContactsStore) Get(ctx context.Context, _ string, id string) (*ContactRecord, error) {
	if err := darwinContactsEnsureAccess(); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, nil
	}

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	pool := newDarwinContactsAutoreleasePool()
	defer releaseDarwinContactsObject(pool)

	store, err := newDarwinContactsStore()
	if err != nil {
		return nil, err
	}
	defer releaseDarwinContactsObject(store)

	keys := darwinContactsKeysArray()
	if keys == 0 {
		return nil, fmt.Errorf("contacts native keys unavailable")
	}
	defer releaseDarwinContactsObject(keys)

	return darwinContactsFetchByIdentifier(store, keys, id)
}

func (s *darwinNativeContactsStore) Create(ctx context.Context, _ string, contact ContactRecord) (*ContactRecord, error) {
	if err := darwinContactsEnsureAccess(); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	pool := newDarwinContactsAutoreleasePool()
	defer releaseDarwinContactsObject(pool)

	store, err := newDarwinContactsStore()
	if err != nil {
		return nil, err
	}
	defer releaseDarwinContactsObject(store)

	keys := darwinContactsKeysArray()
	if keys == 0 {
		return nil, fmt.Errorf("contacts native keys unavailable")
	}
	defer releaseDarwinContactsObject(keys)

	if existing, err := darwinContactsFindExisting(store, keys, contact); err != nil {
		return nil, err
	} else if existing != nil {
		return existing, nil
	}

	contactClass := objc.ID(objc.GetClass("CNMutableContact"))
	if contactClass == 0 {
		return nil, fmt.Errorf("contacts native contact class unavailable")
	}
	nativeContact := contactClass.Send(darwinContactsSelAlloc).Send(darwinContactsSelInit)
	if nativeContact == 0 {
		return nil, fmt.Errorf("contacts native contact allocation failed")
	}
	defer releaseDarwinContactsObject(nativeContact)

	if value := strings.TrimSpace(contact.FirstName); value != "" {
		nativeContact.Send(darwinContactsSelSetGivenName, darwinContactsNSString(value))
	}
	if value := strings.TrimSpace(contact.LastName); value != "" {
		nativeContact.Send(darwinContactsSelSetFamilyName, darwinContactsNSString(value))
	}
	if value := strings.TrimSpace(contact.Organization); value != "" {
		nativeContact.Send(darwinContactsSelSetOrganizationName, darwinContactsNSString(value))
	}
	if value := strings.TrimSpace(contact.Title); value != "" {
		nativeContact.Send(darwinContactsSelSetJobTitle, darwinContactsNSString(value))
	}
	if strings.TrimSpace(contact.FirstName) == "" && strings.TrimSpace(contact.LastName) == "" {
		if displayName := strings.TrimSpace(contact.DisplayName); displayName != "" {
			nativeContact.Send(darwinContactsSelSetGivenName, darwinContactsNSString(displayName))
		}
	}

	if phones := darwinContactsLabeledPhoneArray(contact.PhoneNumbers); phones != 0 {
		nativeContact.Send(darwinContactsSelSetPhoneNumbers, phones)
		releaseDarwinContactsObject(phones)
	}
	if emails := darwinContactsLabeledStringArray(contact.EmailAddresses); emails != 0 {
		nativeContact.Send(darwinContactsSelSetEmailAddresses, emails)
		releaseDarwinContactsObject(emails)
	}
	if addresses := darwinContactsPostalAddressArray(contact.Address); addresses != 0 {
		nativeContact.Send(darwinContactsSelSetPostalAddresses, addresses)
		releaseDarwinContactsObject(addresses)
	}
	if birthday := darwinContactsNSDateComponentsFromBirthday(contact.Birthday); birthday != 0 {
		nativeContact.Send(darwinContactsSelSetBirthday, birthday)
		releaseDarwinContactsObject(birthday)
	}

	saveRequestClass := objc.ID(objc.GetClass("CNSaveRequest"))
	if saveRequestClass == 0 {
		return nil, fmt.Errorf("contacts native save request class unavailable")
	}
	saveRequest := saveRequestClass.Send(darwinContactsSelAlloc).Send(darwinContactsSelInit)
	if saveRequest == 0 {
		return nil, fmt.Errorf("contacts native save request allocation failed")
	}
	defer releaseDarwinContactsObject(saveRequest)

	saveRequest.Send(darwinContactsSelAddContactToContainerWithIdentifier, nativeContact, objc.ID(0))

	var nsErr objc.ID
	ok := objc.Send[bool](store, darwinContactsSelExecuteSaveRequestError, saveRequest, unsafe.Pointer(&nsErr))
	if !ok {
		if nsErr != 0 {
			return nil, fmt.Errorf("contacts native create failed: %s", darwinContactsNSErrorString(nsErr))
		}
		return nil, fmt.Errorf("contacts native create failed")
	}

	identifier := strings.TrimSpace(darwinContactsGoString(objc.Send[objc.ID](nativeContact, darwinContactsSelIdentifier)))
	if identifier != "" {
		return darwinContactsFetchByIdentifier(store, keys, identifier)
	}

	record := darwinContactRecordFromNative(nativeContact)
	return &record, nil
}

func initDarwinNativeContacts() error {
	darwinContactsOnce.Do(func() {
		if _, err := purego.Dlopen("/System/Library/Frameworks/Foundation.framework/Foundation", purego.RTLD_LAZY|purego.RTLD_GLOBAL); err != nil {
			darwinContactsErr = fmt.Errorf("dlopen Foundation: %w", err)
			return
		}
		if _, err := purego.Dlopen("/System/Library/Frameworks/Contacts.framework/Contacts", purego.RTLD_LAZY|purego.RTLD_GLOBAL); err != nil {
			darwinContactsErr = fmt.Errorf("dlopen Contacts: %w", err)
			return
		}

		darwinContactsSelAlloc = objc.RegisterName("alloc")
		darwinContactsSelInit = objc.RegisterName("init")
		darwinContactsSelRelease = objc.RegisterName("release")
		darwinContactsSelStringWithUTF8 = objc.RegisterName("stringWithUTF8String:")
		darwinContactsSelUTF8String = objc.RegisterName("UTF8String")
		darwinContactsSelAuthorizationStatusForEntityType = objc.RegisterName("authorizationStatusForEntityType:")
		darwinContactsSelRequestAccessForEntityTypeCompletion = objc.RegisterName("requestAccessForEntityType:completionHandler:")
		darwinContactsSelEnumerateContactsWithFetchRequestError = objc.RegisterName("enumerateContactsWithFetchRequest:error:usingBlock:")
		darwinContactsSelUnifiedContactWithIdentifierKeysToFetch = objc.RegisterName("unifiedContactWithIdentifier:keysToFetch:error:")
		darwinContactsSelExecuteSaveRequestError = objc.RegisterName("executeSaveRequest:error:")
		darwinContactsSelInitWithKeysToFetch = objc.RegisterName("initWithKeysToFetch:")
		darwinContactsSelAddObject = objc.RegisterName("addObject:")
		darwinContactsSelCount = objc.RegisterName("count")
		darwinContactsSelObjectAtIndex = objc.RegisterName("objectAtIndex:")
		darwinContactsSelIdentifier = objc.RegisterName("identifier")
		darwinContactsSelGivenName = objc.RegisterName("givenName")
		darwinContactsSelFamilyName = objc.RegisterName("familyName")
		darwinContactsSelOrganizationName = objc.RegisterName("organizationName")
		darwinContactsSelJobTitle = objc.RegisterName("jobTitle")
		darwinContactsSelPhoneNumbers = objc.RegisterName("phoneNumbers")
		darwinContactsSelEmailAddresses = objc.RegisterName("emailAddresses")
		darwinContactsSelPostalAddresses = objc.RegisterName("postalAddresses")
		darwinContactsSelBirthday = objc.RegisterName("birthday")
		darwinContactsSelValue = objc.RegisterName("value")
		darwinContactsSelStringValue = objc.RegisterName("stringValue")
		darwinContactsSelStreet = objc.RegisterName("street")
		darwinContactsSelYear = objc.RegisterName("year")
		darwinContactsSelMonth = objc.RegisterName("month")
		darwinContactsSelDay = objc.RegisterName("day")
		darwinContactsSelSetGivenName = objc.RegisterName("setGivenName:")
		darwinContactsSelSetFamilyName = objc.RegisterName("setFamilyName:")
		darwinContactsSelSetOrganizationName = objc.RegisterName("setOrganizationName:")
		darwinContactsSelSetJobTitle = objc.RegisterName("setJobTitle:")
		darwinContactsSelSetPhoneNumbers = objc.RegisterName("setPhoneNumbers:")
		darwinContactsSelSetEmailAddresses = objc.RegisterName("setEmailAddresses:")
		darwinContactsSelSetPostalAddresses = objc.RegisterName("setPostalAddresses:")
		darwinContactsSelSetBirthday = objc.RegisterName("setBirthday:")
		darwinContactsSelSetStreet = objc.RegisterName("setStreet:")
		darwinContactsSelSetYear = objc.RegisterName("setYear:")
		darwinContactsSelSetMonth = objc.RegisterName("setMonth:")
		darwinContactsSelSetDay = objc.RegisterName("setDay:")
		darwinContactsSelPhoneNumberWithStringValue = objc.RegisterName("phoneNumberWithStringValue:")
		darwinContactsSelLabeledValueWithLabelValue = objc.RegisterName("labeledValueWithLabel:value:")
		darwinContactsSelAddContactToContainerWithIdentifier = objc.RegisterName("addContact:toContainerWithIdentifier:")
		darwinContactsSelLocalizedDescription = objc.RegisterName("localizedDescription")
	})
	return darwinContactsErr
}

func checkDarwinContactsAuthorizationRequest() error {
	if err := initDarwinNativeContacts(); err != nil {
		return err
	}
	if !darwinContactsHasUsageDescriptionFunc() {
		return fmt.Errorf("macOS native contacts unavailable: NSContactsUsageDescription missing from Info.plist")
	}
	return nil
}

func darwinContactsEnsureAccess() error {
	if err := initDarwinNativeContacts(); err != nil {
		return err
	}
	status, err := darwinContactsAuthorizationStatusFunc()
	if err != nil {
		return err
	}
	switch status {
	case darwinContactsAuthorizationAuthorized, darwinContactsAuthorizationLimited:
		return nil
	case darwinContactsAuthorizationNotDetermined:
		if err := checkDarwinContactsAuthorizationRequest(); err != nil {
			return err
		}
		granted, err := darwinContactsRequestAccessFunc()
		if err != nil {
			return err
		}
		if granted {
			return nil
		}
	}
	return fmt.Errorf("CONTACTS_PERMISSION_REQUIRED: grant Contacts permission")
}

func newDarwinContactsStore() (objc.ID, error) {
	storeClass := objc.ID(objc.GetClass("CNContactStore"))
	if storeClass == 0 {
		return 0, fmt.Errorf("contacts native store class unavailable")
	}
	store := storeClass.Send(darwinContactsSelAlloc).Send(darwinContactsSelInit)
	if store == 0 {
		return 0, fmt.Errorf("contacts native store unavailable")
	}
	return store, nil
}

func darwinContactsAuthorizationStatus() (int, error) {
	storeClass := objc.ID(objc.GetClass("CNContactStore"))
	if storeClass == 0 {
		return 0, fmt.Errorf("contacts native store class unavailable")
	}
	return objc.Send[int](storeClass, darwinContactsSelAuthorizationStatusForEntityType, uintptr(darwinContactsEntityTypeContacts)), nil
}

func darwinContactsRequestAccess() (bool, error) {
	if err := initDarwinNativeContacts(); err != nil {
		return false, err
	}

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	pool := newDarwinContactsAutoreleasePool()
	defer releaseDarwinContactsObject(pool)

	store, err := newDarwinContactsStore()
	if err != nil {
		return false, err
	}
	defer releaseDarwinContactsObject(store)

	type authResult struct {
		granted bool
		err     error
	}

	authCh := make(chan authResult, 1)
	block := objc.NewBlock(func(_ objc.Block, granted bool, nsErr objc.ID) {
		result := authResult{granted: granted}
		if nsErr != 0 {
			result.err = fmt.Errorf("contacts authorization request failed: %s", darwinContactsNSErrorString(nsErr))
		}
		select {
		case authCh <- result:
		default:
		}
	})
	defer block.Release()

	store.Send(darwinContactsSelRequestAccessForEntityTypeCompletion, uintptr(darwinContactsEntityTypeContacts), block)

	select {
	case result := <-authCh:
		return result.granted, result.err
	case <-time.After(darwinContactsAuthorizationTimeout):
		return false, fmt.Errorf("contacts authorization timed out")
	}
}

func darwinContactsEnumerate(store, keys objc.ID, ctx context.Context, opts ContactsQueryOptions) ([]ContactRecord, error) {
	requestClass := objc.ID(objc.GetClass("CNContactFetchRequest"))
	if requestClass == 0 {
		return nil, fmt.Errorf("contacts native fetch request class unavailable")
	}
	request := requestClass.Send(darwinContactsSelAlloc).Send(darwinContactsSelInitWithKeysToFetch, keys)
	if request == 0 {
		return nil, fmt.Errorf("contacts native fetch request unavailable")
	}
	defer releaseDarwinContactsObject(request)

	result := make([]ContactRecord, 0, 16)
	var nsErr objc.ID
	block := objc.NewBlock(func(_ objc.Block, nativeContact objc.ID, _ unsafe.Pointer) {
		if nativeContact == 0 {
			return
		}
		record := darwinContactRecordFromNative(nativeContact)
		if !contactsMatchQuery(record, opts.Query) {
			return
		}
		result = append(result, record)
	})
	defer block.Release()

	ok := objc.Send[bool](store, darwinContactsSelEnumerateContactsWithFetchRequestError, request, unsafe.Pointer(&nsErr), block)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !ok {
		if nsErr != 0 {
			return nil, fmt.Errorf("contacts native list failed: %s", darwinContactsNSErrorString(nsErr))
		}
		return nil, fmt.Errorf("contacts native list failed")
	}

	sortContactRecords(result)
	limit := opts.Limit
	if limit <= 0 {
		limit = 25
	}
	if len(result) > limit {
		result = result[:limit]
	}
	return result, nil
}

func darwinContactsFetchByIdentifier(store, keys objc.ID, id string) (*ContactRecord, error) {
	var nsErr objc.ID
	nativeContact := objc.Send[objc.ID](store, darwinContactsSelUnifiedContactWithIdentifierKeysToFetch, darwinContactsNSString(id), keys, unsafe.Pointer(&nsErr))
	if nativeContact == 0 {
		if nsErr == 0 {
			return nil, nil
		}
		msg := strings.ToLower(strings.TrimSpace(darwinContactsNSErrorString(nsErr)))
		if strings.Contains(msg, "does not exist") || strings.Contains(msg, "not found") {
			return nil, nil
		}
		return nil, fmt.Errorf("contacts native get failed: %s", darwinContactsNSErrorString(nsErr))
	}
	record := darwinContactRecordFromNative(nativeContact)
	return &record, nil
}

func darwinContactsFindExisting(store, keys objc.ID, candidate ContactRecord) (*ContactRecord, error) {
	if len(candidate.PhoneNumbers) == 0 && len(candidate.EmailAddresses) == 0 {
		return nil, nil
	}
	contacts, err := darwinContactsEnumerate(store, keys, context.Background(), ContactsQueryOptions{Limit: 1000000})
	if err != nil {
		return nil, err
	}
	targetPhones := make(map[string]struct{}, len(candidate.PhoneNumbers))
	for _, phone := range candidate.PhoneNumbers {
		if normalized := normalizeContactPhone(phone); normalized != "" {
			targetPhones[normalized] = struct{}{}
		}
	}
	targetEmails := make(map[string]struct{}, len(candidate.EmailAddresses))
	for _, email := range candidate.EmailAddresses {
		email = strings.ToLower(strings.TrimSpace(email))
		if email != "" {
			targetEmails[email] = struct{}{}
		}
	}
	for _, existing := range contacts {
		if contactMatchesIdentity(existing, targetPhones, targetEmails) {
			match := existing
			return &match, nil
		}
	}
	return nil, nil
}

func contactMatchesIdentity(contact ContactRecord, phones, emails map[string]struct{}) bool {
	for _, phone := range contact.PhoneNumbers {
		if _, ok := phones[normalizeContactPhone(phone)]; ok {
			return true
		}
	}
	for _, email := range contact.EmailAddresses {
		if _, ok := emails[strings.ToLower(strings.TrimSpace(email))]; ok {
			return true
		}
	}
	return false
}

func normalizeContactPhone(phone string) string {
	trimmed := strings.TrimSpace(phone)
	if trimmed == "" {
		return ""
	}
	var digits []rune
	for _, r := range trimmed {
		if r >= '0' && r <= '9' {
			digits = append(digits, r)
		}
	}
	if len(digits) == 0 {
		return trimmed
	}
	return string(digits)
}

func darwinContactRecordFromNative(nativeContact objc.ID) ContactRecord {
	record := ContactRecord{
		ID:           strings.TrimSpace(darwinContactsGoString(objc.Send[objc.ID](nativeContact, darwinContactsSelIdentifier))),
		FirstName:    strings.TrimSpace(darwinContactsGoString(objc.Send[objc.ID](nativeContact, darwinContactsSelGivenName))),
		LastName:     strings.TrimSpace(darwinContactsGoString(objc.Send[objc.ID](nativeContact, darwinContactsSelFamilyName))),
		Organization: strings.TrimSpace(darwinContactsGoString(objc.Send[objc.ID](nativeContact, darwinContactsSelOrganizationName))),
		Title:        strings.TrimSpace(darwinContactsGoString(objc.Send[objc.ID](nativeContact, darwinContactsSelJobTitle))),
		Birthday:     darwinContactsBirthdayString(objc.Send[objc.ID](nativeContact, darwinContactsSelBirthday)),
	}
	record.PhoneNumbers = darwinContactsLabeledValuesAsStrings(objc.Send[objc.ID](nativeContact, darwinContactsSelPhoneNumbers), true)
	record.EmailAddresses = darwinContactsLabeledValuesAsStrings(objc.Send[objc.ID](nativeContact, darwinContactsSelEmailAddresses), false)
	record.Address = darwinContactsPostalAddressString(objc.Send[objc.ID](nativeContact, darwinContactsSelPostalAddresses))
	record.DisplayName = buildContactDisplayName(record)
	if record.DisplayName == "" {
		switch {
		case len(record.EmailAddresses) > 0:
			record.DisplayName = record.EmailAddresses[0]
		case len(record.PhoneNumbers) > 0:
			record.DisplayName = record.PhoneNumbers[0]
		default:
			record.DisplayName = "(unnamed contact)"
		}
	}
	return record
}

func darwinContactsLabeledValuesAsStrings(array objc.ID, phone bool) []string {
	if array == 0 {
		return nil
	}
	count := int(objc.Send[uint64](array, darwinContactsSelCount))
	result := make([]string, 0, count)
	for i := 0; i < count; i++ {
		item := objc.Send[objc.ID](array, darwinContactsSelObjectAtIndex, uintptr(i))
		if item == 0 {
			continue
		}
		value := objc.Send[objc.ID](item, darwinContactsSelValue)
		if value == 0 {
			continue
		}
		var raw string
		if phone {
			raw = darwinContactsGoString(objc.Send[objc.ID](value, darwinContactsSelStringValue))
		} else {
			raw = darwinContactsGoString(value)
		}
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		result = append(result, raw)
	}
	return normalizeContactStringList(result, !phone)
}

func darwinContactsPostalAddressString(array objc.ID) string {
	if array == 0 || objc.Send[uint64](array, darwinContactsSelCount) == 0 {
		return ""
	}
	item := objc.Send[objc.ID](array, darwinContactsSelObjectAtIndex, uintptr(0))
	if item == 0 {
		return ""
	}
	value := objc.Send[objc.ID](item, darwinContactsSelValue)
	if value == 0 {
		return ""
	}
	return strings.TrimSpace(darwinContactsGoString(objc.Send[objc.ID](value, darwinContactsSelStreet)))
}

func darwinContactsBirthdayString(value objc.ID) string {
	if value == 0 {
		return ""
	}
	month := objc.Send[int](value, darwinContactsSelMonth)
	day := objc.Send[int](value, darwinContactsSelDay)
	year := objc.Send[int](value, darwinContactsSelYear)
	if month <= 0 || day <= 0 {
		return ""
	}
	if year > 0 {
		return fmt.Sprintf("%04d-%02d-%02d", year, month, day)
	}
	return fmt.Sprintf("%02d-%02d", month, day)
}

func darwinContactsKeysArray() objc.ID {
	arrayClass := objc.ID(objc.GetClass("NSMutableArray"))
	if arrayClass == 0 {
		return 0
	}
	array := arrayClass.Send(darwinContactsSelAlloc).Send(darwinContactsSelInit)
	for _, key := range []string{
		"identifier",
		"givenName",
		"familyName",
		"organizationName",
		"jobTitle",
		"phoneNumbers",
		"emailAddresses",
		"postalAddresses",
		"birthday",
	} {
		if nsKey := darwinContactsNSString(key); nsKey != 0 {
			array.Send(darwinContactsSelAddObject, nsKey)
		}
	}
	return array
}

func darwinContactsLabeledPhoneArray(values []string) objc.ID {
	return darwinContactsLabeledValueArray(values, func(value string) objc.ID {
		phoneClass := objc.ID(objc.GetClass("CNPhoneNumber"))
		if phoneClass == 0 {
			return 0
		}
		return phoneClass.Send(darwinContactsSelPhoneNumberWithStringValue, darwinContactsNSString(value))
	})
}

func darwinContactsLabeledStringArray(values []string) objc.ID {
	return darwinContactsLabeledValueArray(values, func(value string) objc.ID {
		return darwinContactsNSString(value)
	})
}

func darwinContactsPostalAddressArray(value string) objc.ID {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	addressClass := objc.ID(objc.GetClass("CNMutablePostalAddress"))
	if addressClass == 0 {
		return 0
	}
	address := addressClass.Send(darwinContactsSelAlloc).Send(darwinContactsSelInit)
	if address == 0 {
		return 0
	}
	address.Send(darwinContactsSelSetStreet, darwinContactsNSString(value))
	defer releaseDarwinContactsObject(address)

	return darwinContactsLabeledValueArray([]string{value}, func(string) objc.ID {
		return address
	})
}

func darwinContactsLabeledValueArray(values []string, transform func(string) objc.ID) objc.ID {
	values = normalizeStringList(values)
	if len(values) == 0 {
		return 0
	}
	arrayClass := objc.ID(objc.GetClass("NSMutableArray"))
	labelClass := objc.ID(objc.GetClass("CNLabeledValue"))
	if arrayClass == 0 || labelClass == 0 {
		return 0
	}
	array := arrayClass.Send(darwinContactsSelAlloc).Send(darwinContactsSelInit)
	for _, value := range values {
		nativeValue := transform(strings.TrimSpace(value))
		if nativeValue == 0 {
			continue
		}
		labeled := labelClass.Send(darwinContactsSelLabeledValueWithLabelValue, objc.ID(0), nativeValue)
		if labeled == 0 {
			continue
		}
		array.Send(darwinContactsSelAddObject, labeled)
	}
	if objc.Send[uint64](array, darwinContactsSelCount) == 0 {
		releaseDarwinContactsObject(array)
		return 0
	}
	return array
}

func darwinContactsNSDateComponentsFromBirthday(raw string) objc.ID {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0
	}
	birthday, err := time.Parse("2006-01-02", raw)
	if err != nil {
		return 0
	}
	class := objc.ID(objc.GetClass("NSDateComponents"))
	if class == 0 {
		return 0
	}
	value := class.Send(darwinContactsSelAlloc).Send(darwinContactsSelInit)
	if value == 0 {
		return 0
	}
	value.Send(darwinContactsSelSetYear, birthday.Year())
	value.Send(darwinContactsSelSetMonth, int(birthday.Month()))
	value.Send(darwinContactsSelSetDay, birthday.Day())
	return value
}

func newDarwinContactsAutoreleasePool() objc.ID {
	poolClass := objc.ID(objc.GetClass("NSAutoreleasePool"))
	if poolClass == 0 {
		return 0
	}
	return poolClass.Send(darwinContactsSelAlloc).Send(darwinContactsSelInit)
}

func releaseDarwinContactsObject(id objc.ID) {
	if id == 0 {
		return
	}
	id.Send(darwinContactsSelRelease)
}

func darwinContactsNSString(value string) objc.ID {
	cstr := append([]byte(value), 0)
	cls := objc.ID(objc.GetClass("NSString"))
	if cls == 0 || len(cstr) == 0 {
		return 0
	}
	return cls.Send(darwinContactsSelStringWithUTF8, uintptr(unsafe.Pointer(&cstr[0])))
}

func darwinContactsGoString(nsStr objc.ID) string {
	if nsStr == 0 {
		return ""
	}
	ptr := objc.Send[uintptr](nsStr, darwinContactsSelUTF8String)
	if ptr == 0 {
		return ""
	}
	return darwinContactsCString(ptr)
}

func darwinContactsCString(ptr uintptr) string {
	if ptr == 0 {
		return ""
	}
	var out []byte
	for {
		b := *(*byte)(unsafe.Pointer(ptr))
		if b == 0 {
			break
		}
		out = append(out, b)
		ptr++
	}
	return string(out)
}

func darwinContactsNSErrorString(nsErr objc.ID) string {
	if nsErr == 0 {
		return ""
	}
	if described := objc.Send[objc.ID](nsErr, darwinContactsSelLocalizedDescription); described != 0 {
		return strings.TrimSpace(darwinContactsGoString(described))
	}
	return "unknown native contacts error"
}

func darwinContactsHasUsageDescription(keyName string) bool {
	if err := initDarwinNativeContacts(); err != nil {
		return false
	}
	cls := objc.ID(objc.GetClass("NSBundle"))
	if cls == 0 {
		return false
	}
	selMainBundle := objc.RegisterName("mainBundle")
	selObjectForInfoDictionaryKey := objc.RegisterName("objectForInfoDictionaryKey:")
	bundle := cls.Send(selMainBundle)
	if bundle == 0 {
		return false
	}
	value := bundle.Send(selObjectForInfoDictionaryKey, darwinContactsNSString(keyName))
	return value != 0
}
