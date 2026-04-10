//go:build darwin

package tools

import (
	"context"
	"fmt"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"
)

const (
	darwinCalendarEntityTypeEvent = 0

	darwinCalendarAuthorizationNotDetermined = 0
	darwinCalendarAuthorizationRestricted    = 1
	darwinCalendarAuthorizationDenied        = 2
	darwinCalendarAuthorizationFullAccess    = 3
	darwinCalendarAuthorizationWriteOnly     = 4
	darwinCalendarSpanThisEvent              = 0

	darwinCalendarStatusNone      = 0
	darwinCalendarStatusConfirmed = 1
	darwinCalendarStatusTentative = 2
	darwinCalendarStatusCanceled  = 3
)

const darwinCalendarAuthorizationTimeout = 2 * time.Minute

var (
	darwinCalendarOnce sync.Once
	darwinCalendarErr  error

	darwinCalendarSelAlloc                                           objc.SEL
	darwinCalendarSelInit                                            objc.SEL
	darwinCalendarSelRelease                                         objc.SEL
	darwinCalendarSelRespondsToSelector                              objc.SEL
	darwinCalendarSelStringWithUTF8                                  objc.SEL
	darwinCalendarSelUTF8String                                      objc.SEL
	darwinCalendarSelAuthorizationStatusForEntityType                objc.SEL
	darwinCalendarSelRequestAccessToEntityTypeCompletion             objc.SEL
	darwinCalendarSelRequestFullAccessToEventsWithCompletion         objc.SEL
	darwinCalendarSelPredicateForEventsWithStartDateEndDateCalendars objc.SEL
	darwinCalendarSelEventsMatchingPredicate                         objc.SEL
	darwinCalendarSelEventWithIdentifier                             objc.SEL
	darwinCalendarSelCalendarsForEntityType                          objc.SEL
	darwinCalendarSelDefaultCalendarForNewEvents                     objc.SEL
	darwinCalendarSelCalendarWithIdentifier                          objc.SEL
	darwinCalendarSelObjectAtIndex                                   objc.SEL
	darwinCalendarSelCount                                           objc.SEL
	darwinCalendarSelTitle                                           objc.SEL
	darwinCalendarSelLocation                                        objc.SEL
	darwinCalendarSelNotes                                           objc.SEL
	darwinCalendarSelCalendar                                        objc.SEL
	darwinCalendarSelEventIdentifier                                 objc.SEL
	darwinCalendarSelStartDate                                       objc.SEL
	darwinCalendarSelEndDate                                         objc.SEL
	darwinCalendarSelIsAllDay                                        objc.SEL
	darwinCalendarSelStatus                                          objc.SEL
	darwinCalendarSelCreationDate                                    objc.SEL
	darwinCalendarSelLastModifiedDate                                objc.SEL
	darwinCalendarSelTimeIntervalSince1970                           objc.SEL
	darwinCalendarSelDateWithTimeIntervalSince1970                   objc.SEL
	darwinCalendarSelEventWithEventStore                             objc.SEL
	darwinCalendarSelSetTitle                                        objc.SEL
	darwinCalendarSelSetStartDate                                    objc.SEL
	darwinCalendarSelSetEndDate                                      objc.SEL
	darwinCalendarSelSetAllDay                                       objc.SEL
	darwinCalendarSelSetLocation                                     objc.SEL
	darwinCalendarSelSetNotes                                        objc.SEL
	darwinCalendarSelSetCalendar                                     objc.SEL
	darwinCalendarSelSaveEventSpanError                              objc.SEL
	darwinCalendarSelLocalizedDescription                            objc.SEL

	darwinCalendarSupportsFullAccessRequestFunc     = darwinCalendarSupportsFullAccessRequest
	darwinCalendarHasFullAccessUsageDescriptionFunc = func() bool {
		return darwinCalendarHasUsageDescription("NSCalendarsFullAccessUsageDescription")
	}
	darwinCalendarHasLegacyUsageDescriptionFunc = func() bool {
		return darwinCalendarHasUsageDescription("NSCalendarsUsageDescription")
	}
	darwinCalendarAuthorizationStatusFunc = darwinCalendarAuthorizationStatus
	darwinCalendarRequestFullAccessFunc   = darwinCalendarRequestAccess
)

type darwinNativeCalendarStore struct{}

func newDarwinNativeCalendarStoreOrNil(_ CalendarStore) CalendarStore {
	if err := initDarwinNativeCalendar(); err != nil {
		return nil
	}
	if objc.GetClass("EKEventStore") == 0 || objc.GetClass("EKEvent") == 0 || objc.GetClass("NSDate") == 0 {
		return nil
	}
	return &darwinNativeCalendarStore{}
}

func (s *darwinNativeCalendarStore) List(ctx context.Context, _ string, opts CalendarQueryOptions) ([]CalendarEvent, error) {
	if err := darwinCalendarCheckReadAccess(); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	pool := newDarwinCalendarAutoreleasePool()
	defer releaseDarwinCalendarObject(pool)

	store, err := newDarwinCalendarEventStore()
	if err != nil {
		return nil, err
	}
	defer releaseDarwinCalendarObject(store)

	start, end := darwinCalendarQueryRange(opts, time.Now())
	predicate := objc.Send[objc.ID](store,
		darwinCalendarSelPredicateForEventsWithStartDateEndDateCalendars,
		darwinCalendarNSDate(start),
		darwinCalendarNSDate(end),
		objc.ID(0),
	)
	if predicate == 0 {
		return nil, fmt.Errorf("calendar native query predicate unavailable")
	}

	array := objc.Send[objc.ID](store, darwinCalendarSelEventsMatchingPredicate, predicate)
	if array == 0 {
		return nil, nil
	}
	count := int(objc.Send[uint64](array, darwinCalendarSelCount))
	result := make([]CalendarEvent, 0, count)
	for i := 0; i < count; i++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		item := objc.Send[objc.ID](array, darwinCalendarSelObjectAtIndex, uintptr(i))
		if item == 0 {
			continue
		}
		event := darwinCalendarEventFromNative(item)
		if !calendarMatchesQuery(event, opts) {
			continue
		}
		result = append(result, event)
	}

	sort.SliceStable(result, func(i, j int) bool {
		if result[i].StartAt.Equal(result[j].StartAt) {
			return result[i].Title < result[j].Title
		}
		return result[i].StartAt.Before(result[j].StartAt)
	})
	limit := opts.Limit
	if limit <= 0 {
		limit = 10
	}
	if len(result) > limit {
		result = result[:limit]
	}
	return result, nil
}

func (s *darwinNativeCalendarStore) Get(ctx context.Context, _ string, id string) (*CalendarEvent, error) {
	if err := darwinCalendarCheckReadAccess(); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	trimmedID := strings.TrimSpace(id)
	if trimmedID == "" {
		return nil, nil
	}

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	pool := newDarwinCalendarAutoreleasePool()
	defer releaseDarwinCalendarObject(pool)

	store, err := newDarwinCalendarEventStore()
	if err != nil {
		return nil, err
	}
	defer releaseDarwinCalendarObject(store)

	item := objc.Send[objc.ID](store, darwinCalendarSelEventWithIdentifier, darwinCalendarNSString(trimmedID))
	if item == 0 {
		return nil, nil
	}
	event := darwinCalendarEventFromNative(item)
	return &event, nil
}

func (s *darwinNativeCalendarStore) Create(ctx context.Context, _ string, event CalendarEvent) (*CalendarEvent, error) {
	if err := darwinCalendarCheckWriteAccess(); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	pool := newDarwinCalendarAutoreleasePool()
	defer releaseDarwinCalendarObject(pool)

	store, err := newDarwinCalendarEventStore()
	if err != nil {
		return nil, err
	}
	defer releaseDarwinCalendarObject(store)

	eventClass := objc.ID(objc.GetClass("EKEvent"))
	if eventClass == 0 {
		return nil, fmt.Errorf("calendar native event class unavailable")
	}
	nativeEvent := objc.Send[objc.ID](eventClass, darwinCalendarSelEventWithEventStore, store)
	if nativeEvent == 0 {
		return nil, fmt.Errorf("calendar native event allocation failed")
	}

	nativeEvent.Send(darwinCalendarSelSetTitle, darwinCalendarNSString(strings.TrimSpace(event.Title)))
	nativeEvent.Send(darwinCalendarSelSetStartDate, darwinCalendarNSDate(event.StartAt))
	nativeEvent.Send(darwinCalendarSelSetEndDate, darwinCalendarNSDate(event.EndAt))
	nativeEvent.Send(darwinCalendarSelSetAllDay, event.AllDay)
	if location := strings.TrimSpace(event.Location); location != "" {
		nativeEvent.Send(darwinCalendarSelSetLocation, darwinCalendarNSString(location))
	}
	if notes := strings.TrimSpace(event.Notes); notes != "" {
		nativeEvent.Send(darwinCalendarSelSetNotes, darwinCalendarNSString(notes))
	}

	calendar, err := darwinCalendarResolveTargetCalendar(store, event.CalendarName)
	if err != nil {
		return nil, err
	}
	nativeEvent.Send(darwinCalendarSelSetCalendar, calendar)

	var nsErr objc.ID
	ok := objc.Send[bool](store, darwinCalendarSelSaveEventSpanError, nativeEvent, uintptr(darwinCalendarSpanThisEvent), unsafe.Pointer(&nsErr))
	if !ok {
		if nsErr != 0 {
			return nil, fmt.Errorf("calendar native create failed: %s", darwinCalendarNSErrorString(nsErr))
		}
		return nil, fmt.Errorf("calendar native create failed")
	}

	created := darwinCalendarEventFromNative(nativeEvent)
	return &created, nil
}

func (s *darwinNativeCalendarStore) Today(ctx context.Context, ownerID string, day time.Time) ([]CalendarEvent, error) {
	start, end := dayBounds(day)
	return s.List(ctx, ownerID, CalendarQueryOptions{
		Start: &start,
		End:   &end,
		Limit: 20,
	})
}

func initDarwinNativeCalendar() error {
	darwinCalendarOnce.Do(func() {
		if _, err := purego.Dlopen("/System/Library/Frameworks/Foundation.framework/Foundation", purego.RTLD_LAZY|purego.RTLD_GLOBAL); err != nil {
			darwinCalendarErr = fmt.Errorf("dlopen Foundation: %w", err)
			return
		}
		if _, err := purego.Dlopen("/System/Library/Frameworks/EventKit.framework/EventKit", purego.RTLD_LAZY|purego.RTLD_GLOBAL); err != nil {
			darwinCalendarErr = fmt.Errorf("dlopen EventKit: %w", err)
			return
		}

		darwinCalendarSelAlloc = objc.RegisterName("alloc")
		darwinCalendarSelInit = objc.RegisterName("init")
		darwinCalendarSelRelease = objc.RegisterName("release")
		darwinCalendarSelRespondsToSelector = objc.RegisterName("respondsToSelector:")
		darwinCalendarSelStringWithUTF8 = objc.RegisterName("stringWithUTF8String:")
		darwinCalendarSelUTF8String = objc.RegisterName("UTF8String")
		darwinCalendarSelAuthorizationStatusForEntityType = objc.RegisterName("authorizationStatusForEntityType:")
		darwinCalendarSelRequestAccessToEntityTypeCompletion = objc.RegisterName("requestAccessToEntityType:completion:")
		darwinCalendarSelRequestFullAccessToEventsWithCompletion = objc.RegisterName("requestFullAccessToEventsWithCompletion:")
		darwinCalendarSelPredicateForEventsWithStartDateEndDateCalendars = objc.RegisterName("predicateForEventsWithStartDate:endDate:calendars:")
		darwinCalendarSelEventsMatchingPredicate = objc.RegisterName("eventsMatchingPredicate:")
		darwinCalendarSelEventWithIdentifier = objc.RegisterName("eventWithIdentifier:")
		darwinCalendarSelCalendarsForEntityType = objc.RegisterName("calendarsForEntityType:")
		darwinCalendarSelDefaultCalendarForNewEvents = objc.RegisterName("defaultCalendarForNewEvents")
		darwinCalendarSelCalendarWithIdentifier = objc.RegisterName("calendarWithIdentifier:")
		darwinCalendarSelObjectAtIndex = objc.RegisterName("objectAtIndex:")
		darwinCalendarSelCount = objc.RegisterName("count")
		darwinCalendarSelTitle = objc.RegisterName("title")
		darwinCalendarSelLocation = objc.RegisterName("location")
		darwinCalendarSelNotes = objc.RegisterName("notes")
		darwinCalendarSelCalendar = objc.RegisterName("calendar")
		darwinCalendarSelEventIdentifier = objc.RegisterName("eventIdentifier")
		darwinCalendarSelStartDate = objc.RegisterName("startDate")
		darwinCalendarSelEndDate = objc.RegisterName("endDate")
		darwinCalendarSelIsAllDay = objc.RegisterName("isAllDay")
		darwinCalendarSelStatus = objc.RegisterName("status")
		darwinCalendarSelCreationDate = objc.RegisterName("creationDate")
		darwinCalendarSelLastModifiedDate = objc.RegisterName("lastModifiedDate")
		darwinCalendarSelTimeIntervalSince1970 = objc.RegisterName("timeIntervalSince1970")
		darwinCalendarSelDateWithTimeIntervalSince1970 = objc.RegisterName("dateWithTimeIntervalSince1970:")
		darwinCalendarSelEventWithEventStore = objc.RegisterName("eventWithEventStore:")
		darwinCalendarSelSetTitle = objc.RegisterName("setTitle:")
		darwinCalendarSelSetStartDate = objc.RegisterName("setStartDate:")
		darwinCalendarSelSetEndDate = objc.RegisterName("setEndDate:")
		darwinCalendarSelSetAllDay = objc.RegisterName("setAllDay:")
		darwinCalendarSelSetLocation = objc.RegisterName("setLocation:")
		darwinCalendarSelSetNotes = objc.RegisterName("setNotes:")
		darwinCalendarSelSetCalendar = objc.RegisterName("setCalendar:")
		darwinCalendarSelSaveEventSpanError = objc.RegisterName("saveEvent:span:error:")
		darwinCalendarSelLocalizedDescription = objc.RegisterName("localizedDescription")
	})
	return darwinCalendarErr
}

func checkDarwinCalendarAuthorizationRequest() error {
	if err := initDarwinNativeCalendar(); err != nil {
		return err
	}
	if darwinCalendarSupportsFullAccessRequestFunc() {
		if !darwinCalendarHasFullAccessUsageDescriptionFunc() {
			return fmt.Errorf("macOS native calendar unavailable: " +
				"NSCalendarsFullAccessUsageDescription missing from Info.plist")
		}
		return nil
	}
	if !darwinCalendarHasLegacyUsageDescriptionFunc() {
		return fmt.Errorf("macOS native calendar unavailable: " +
			"NSCalendarsUsageDescription missing from Info.plist")
	}
	return nil
}

func darwinCalendarCheckReadAccess() error {
	return darwinCalendarEnsureAccess(false)
}

func darwinCalendarCheckWriteAccess() error {
	return darwinCalendarEnsureAccess(true)
}

func darwinCalendarEnsureAccess(write bool) error {
	if err := initDarwinNativeCalendar(); err != nil {
		return err
	}
	status, err := darwinCalendarAuthorizationStatusFunc()
	if err != nil {
		return err
	}
	switch status {
	case darwinCalendarAuthorizationFullAccess:
		return nil
	case darwinCalendarAuthorizationWriteOnly:
		if write {
			return nil
		}
	case darwinCalendarAuthorizationNotDetermined:
		if err := checkDarwinCalendarAuthorizationRequest(); err != nil {
			return err
		}
		granted, err := darwinCalendarRequestFullAccessFunc()
		if err != nil {
			return err
		}
		if granted {
			return nil
		}
		return fmt.Errorf("CALENDAR_PERMISSION_REQUIRED: grant Calendar permission")
	default:
		return fmt.Errorf("CALENDAR_PERMISSION_REQUIRED: grant Calendar permission")
	}
	return fmt.Errorf("CALENDAR_PERMISSION_REQUIRED: grant Calendar permission")
}

func newDarwinCalendarEventStore() (objc.ID, error) {
	storeClass := objc.ID(objc.GetClass("EKEventStore"))
	if storeClass == 0 {
		return 0, fmt.Errorf("calendar native event store class unavailable")
	}
	store := storeClass.Send(darwinCalendarSelAlloc).Send(darwinCalendarSelInit)
	if store == 0 {
		return 0, fmt.Errorf("calendar native event store unavailable")
	}
	return store, nil
}

func darwinCalendarAuthorizationStatus() (int, error) {
	storeClass := objc.ID(objc.GetClass("EKEventStore"))
	if storeClass == 0 {
		return 0, fmt.Errorf("calendar native event store class unavailable")
	}
	status := objc.Send[int](storeClass, darwinCalendarSelAuthorizationStatusForEntityType, uintptr(darwinCalendarEntityTypeEvent))
	return status, nil
}

func darwinCalendarSupportsFullAccessRequest() bool {
	if err := initDarwinNativeCalendar(); err != nil {
		return false
	}

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	pool := newDarwinCalendarAutoreleasePool()
	defer releaseDarwinCalendarObject(pool)

	store, err := newDarwinCalendarEventStore()
	if err != nil {
		return false
	}
	defer releaseDarwinCalendarObject(store)

	return objc.Send[bool](store, darwinCalendarSelRespondsToSelector, darwinCalendarSelRequestFullAccessToEventsWithCompletion)
}

func darwinCalendarRequestAccess() (bool, error) {
	if err := initDarwinNativeCalendar(); err != nil {
		return false, err
	}

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	pool := newDarwinCalendarAutoreleasePool()
	defer releaseDarwinCalendarObject(pool)

	store, err := newDarwinCalendarEventStore()
	if err != nil {
		return false, err
	}
	defer releaseDarwinCalendarObject(store)

	type authResult struct {
		granted bool
		err     error
	}

	authCh := make(chan authResult, 1)
	block := objc.NewBlock(func(_ objc.Block, granted bool, nsErr objc.ID) {
		result := authResult{granted: granted}
		if nsErr != 0 {
			result.err = fmt.Errorf("calendar authorization request failed: %s", darwinCalendarNSErrorString(nsErr))
		}
		select {
		case authCh <- result:
		default:
		}
	})
	defer block.Release()

	if darwinCalendarSupportsFullAccessRequestFunc() {
		store.Send(darwinCalendarSelRequestFullAccessToEventsWithCompletion, block)
	} else {
		store.Send(darwinCalendarSelRequestAccessToEntityTypeCompletion, uintptr(darwinCalendarEntityTypeEvent), block)
	}

	select {
	case result := <-authCh:
		return result.granted, result.err
	case <-time.After(darwinCalendarAuthorizationTimeout):
		return false, fmt.Errorf("calendar authorization timed out")
	}
}

func darwinCalendarQueryRange(opts CalendarQueryOptions, now time.Time) (time.Time, time.Time) {
	start := now
	end := now.Add(7 * 24 * time.Hour)
	if opts.Start != nil {
		start = *opts.Start
		if opts.End == nil {
			end = start.Add(7 * 24 * time.Hour)
		}
	}
	if opts.End != nil {
		end = *opts.End
		if opts.Start == nil {
			start = now
		}
	}
	if !end.After(start) {
		end = start.Add(time.Hour)
	}
	return start, end
}

func darwinCalendarResolveTargetCalendar(store objc.ID, calendarName string) (objc.ID, error) {
	trimmed := strings.TrimSpace(calendarName)
	if trimmed == "" {
		calendar := objc.Send[objc.ID](store, darwinCalendarSelDefaultCalendarForNewEvents)
		if calendar == 0 {
			return 0, fmt.Errorf("CALENDAR_NOT_FOUND: no default calendar")
		}
		return calendar, nil
	}

	array := objc.Send[objc.ID](store, darwinCalendarSelCalendarsForEntityType, uintptr(darwinCalendarEntityTypeEvent))
	if array == 0 {
		return 0, fmt.Errorf("CALENDAR_NOT_FOUND: no calendar named %s", trimmed)
	}
	count := int(objc.Send[uint64](array, darwinCalendarSelCount))
	for i := 0; i < count; i++ {
		calendar := objc.Send[objc.ID](array, darwinCalendarSelObjectAtIndex, uintptr(i))
		if calendar == 0 {
			continue
		}
		title := darwinCalendarGoString(objc.Send[objc.ID](calendar, darwinCalendarSelTitle))
		if strings.EqualFold(strings.TrimSpace(title), trimmed) {
			return calendar, nil
		}
	}
	return 0, fmt.Errorf("CALENDAR_NOT_FOUND: no calendar named %s", trimmed)
}

func darwinCalendarEventFromNative(event objc.ID) CalendarEvent {
	startAt := darwinCalendarNSDateToTime(objc.Send[objc.ID](event, darwinCalendarSelStartDate))
	endAt := darwinCalendarNSDateToTime(objc.Send[objc.ID](event, darwinCalendarSelEndDate))
	if endAt.IsZero() {
		endAt = startAt
	}
	createdAt := darwinCalendarNSDateToTime(objc.Send[objc.ID](event, darwinCalendarSelCreationDate))
	updatedAt := darwinCalendarNSDateToTime(objc.Send[objc.ID](event, darwinCalendarSelLastModifiedDate))
	calendar := objc.Send[objc.ID](event, darwinCalendarSelCalendar)
	status := darwinCalendarStatusString(objc.Send[int](event, darwinCalendarSelStatus))
	title := strings.TrimSpace(darwinCalendarGoString(objc.Send[objc.ID](event, darwinCalendarSelTitle)))
	if title == "" {
		title = "(untitled)"
	}
	return CalendarEvent{
		ID:           strings.TrimSpace(darwinCalendarGoString(objc.Send[objc.ID](event, darwinCalendarSelEventIdentifier))),
		Title:        title,
		Location:     strings.TrimSpace(darwinCalendarGoString(objc.Send[objc.ID](event, darwinCalendarSelLocation))),
		Notes:        strings.TrimSpace(darwinCalendarGoString(objc.Send[objc.ID](event, darwinCalendarSelNotes))),
		CalendarName: strings.TrimSpace(darwinCalendarGoString(objc.Send[objc.ID](calendar, darwinCalendarSelTitle))),
		Status:       status,
		StartAt:      startAt,
		EndAt:        endAt,
		AllDay:       objc.Send[bool](event, darwinCalendarSelIsAllDay),
		CreatedAt:    createdAt,
		UpdatedAt:    updatedAt,
	}
}

func darwinCalendarStatusString(status int) string {
	switch status {
	case darwinCalendarStatusTentative:
		return "tentative"
	case darwinCalendarStatusCanceled:
		return "cancelled"
	case darwinCalendarStatusConfirmed:
		return "confirmed"
	default:
		return "confirmed"
	}
}

func newDarwinCalendarAutoreleasePool() objc.ID {
	poolClass := objc.ID(objc.GetClass("NSAutoreleasePool"))
	if poolClass == 0 {
		return 0
	}
	return poolClass.Send(darwinCalendarSelAlloc).Send(darwinCalendarSelInit)
}

func releaseDarwinCalendarObject(id objc.ID) {
	if id == 0 {
		return
	}
	id.Send(darwinCalendarSelRelease)
}

func darwinCalendarNSString(value string) objc.ID {
	cstr := append([]byte(value), 0)
	cls := objc.ID(objc.GetClass("NSString"))
	if cls == 0 {
		return 0
	}
	return cls.Send(darwinCalendarSelStringWithUTF8, uintptr(unsafe.Pointer(&cstr[0])))
}

func darwinCalendarGoString(nsStr objc.ID) string {
	if nsStr == 0 {
		return ""
	}
	ptr := objc.Send[uintptr](nsStr, darwinCalendarSelUTF8String)
	if ptr == 0 {
		return ""
	}
	return darwinCalendarCString(ptr)
}

func darwinCalendarCString(ptr uintptr) string {
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

func darwinCalendarNSDate(value time.Time) objc.ID {
	dateClass := objc.ID(objc.GetClass("NSDate"))
	if dateClass == 0 {
		return 0
	}
	return objc.Send[objc.ID](dateClass, darwinCalendarSelDateWithTimeIntervalSince1970, value.UTC().Sub(time.Unix(0, 0)).Seconds())
}

func darwinCalendarNSDateToTime(value objc.ID) time.Time {
	if value == 0 {
		return time.Time{}
	}
	seconds := objc.Send[float64](value, darwinCalendarSelTimeIntervalSince1970)
	whole, frac := mathModf(seconds)
	return time.Unix(int64(whole), int64(frac*float64(time.Second))).UTC()
}

func darwinCalendarNSErrorString(nsErr objc.ID) string {
	if nsErr == 0 {
		return ""
	}
	if described := objc.Send[objc.ID](nsErr, darwinCalendarSelLocalizedDescription); described != 0 {
		return strings.TrimSpace(darwinCalendarGoString(described))
	}
	return "unknown native calendar error"
}

func darwinCalendarHasUsageDescription(keyName string) bool {
	if err := initDarwinNativeCalendar(); err != nil {
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
	value := bundle.Send(selObjectForInfoDictionaryKey, darwinCalendarNSString(keyName))
	return value != 0
}

func mathModf(value float64) (float64, float64) {
	whole := float64(int64(value))
	return whole, value - whole
}
