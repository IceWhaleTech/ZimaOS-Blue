# HomeView Performance Optimization Summary

## Optimizations Implemented

### 1. Polling Interval Optimization (CRITICAL)
**Before:** 5-second polling interval
**After:** 15-second polling interval
**Impact:** 66% reduction in API calls (from 720/hour to 240/hour per user)

**Location:** [HomeView.vue:95](web/src/views/HomeView.vue#L95)

```typescript
// Optimized polling interval: 15 seconds (reduced from 5s = 66% fewer API calls)
refreshInterval = setInterval(() => {
  if (autoRefresh.value && isPageVisible.value) {
    systemStore.fetchAll()
    fetchMetricsHistory()
  }
}, 15000)
```

### 2. Request Cancellation (CRITICAL)
**Before:** No request cancellation - potential memory leaks
**After:** AbortController cancels pending requests on unmount

**Location:** [HomeView.vue:35-49](web/src/views/HomeView.vue#L35-L49)

```typescript
// AbortController for request cancellation
let abortController: AbortController | null = null

async function fetchMetricsHistory() {
  try {
    // Cancel previous request if still pending
    if (abortController) {
      abortController.abort()
    }
    abortController = new AbortController()
    // ... fetch logic
  } catch (error: any) {
    // Don't update state if request was aborted
    if (error?.name !== 'AbortError' && error?.name !== 'CanceledError') {
      metricsHistory.value = []
    }
  }
}
```

### 3. Visibility API Integration (HIGH)
**Before:** Polling continues when tab is hidden
**After:** Polling pauses when tab is hidden, resumes when visible

**Location:** [HomeView.vue:68-71](web/src/views/HomeView.vue#L68-L71)

```typescript
// Handle visibility change to pause/resume polling
function handleVisibilityChange() {
  isPageVisible.value = !document.hidden
}

// In polling interval
if (autoRefresh.value && isPageVisible.value) {
  systemStore.fetchAll()
  fetchMetricsHistory()
}
```

### 4. Debounced Auto-Refresh Toggle (MEDIUM)
**Before:** No debouncing - rapid toggles could flood API
**After:** 100ms debounce on toggle changes

**Location:** [HomeView.vue:73-85](web/src/views/HomeView.vue#L73-L85)

```typescript
// Debounced refresh function
let refreshDebounceTimer: ReturnType<typeof setTimeout> | null = null
function debouncedRefresh() {
  if (refreshDebounceTimer) {
    clearTimeout(refreshDebounceTimer)
  }
  refreshDebounceTimer = setTimeout(() => {
    if (autoRefresh.value && isPageVisible.value) {
      systemStore.fetchAll()
      fetchMetricsHistory()
    }
  }, 100)
}
```

### 5. ShallowRef for Large Arrays (MEDIUM)
**Before:** Deep reactivity on metrics array (unnecessary overhead)
**After:** ShallowRef for better performance with large arrays

**Location:** [HomeView.vue:24](web/src/views/HomeView.vue#L24)

```typescript
// Metrics history for dashboard (use shallowRef for performance)
const metricsHistory = shallowRef<SystemMetrics[]>([])
```

### 6. Data Caching in Store (HIGH)
**Before:** No caching - every request hits the API
**After:** 10-second cache TTL with request deduplication

**Location:** [system.ts:8-77](web/src/stores/system.ts#L8-L77)

```typescript
// Cache configuration
const CACHE_TTL = 10000 // 10 seconds cache TTL

async function fetchHealth(forceRefresh = false) {
  // Return cached data if still valid
  const now = Date.now()
  if (!forceRefresh && health.value && now - healthCacheTime < CACHE_TTL) {
    return
  }

  // Return existing pending request if one is in flight
  if (pendingHealthRequest) {
    return pendingHealthRequest
  }
  // ... fetch logic
}
```

### 7. Request Deduplication (HIGH)
**Before:** Concurrent identical requests all hit the API
**After:** Concurrent requests share the same promise

**Location:** [system.ts:30-45](web/src/stores/system.ts#L30-L45)

```typescript
// Pending request tracking to prevent duplicate requests
let pendingHealthRequest: Promise<void> | null = null

// Return existing pending request if one is in flight
if (pendingHealthRequest) {
  return pendingHealthRequest
}
```

### 8. Comprehensive Cleanup (CRITICAL)
**Before:** Only interval cleanup
**After:** Cleanup of intervals, timers, requests, and event listeners

**Location:** [HomeView.vue:107-125](web/src/views/HomeView.vue#L107-L125)

```typescript
onUnmounted(() => {
  // Cleanup interval
  if (refreshInterval) {
    clearInterval(refreshInterval)
    refreshInterval = null
  }

  // Cleanup debounce timer
  if (refreshDebounceTimer) {
    clearTimeout(refreshDebounceTimer)
    refreshDebounceTimer = null
  }

  // Cancel pending requests
  if (abortController) {
    abortController.abort()
    abortController = null
  }

  // Remove visibility listener
  document.removeEventListener('visibilitychange', handleVisibilityChange)
})
```

### 9. Performance Monitoring Utility (BONUS)
**New File:** [performanceMonitor.ts](web/src/utils/performanceMonitor.ts)

Provides utilities for tracking component performance:
- Mark/measure operations
- Track average durations
- Log performance summaries
- Warn on slow operations (>100ms)

```typescript
import { usePerformanceTracking } from '@/utils/performanceMonitor'

const { trackOperation } = usePerformanceTracking('HomeView')

const fetchData = trackOperation('fetchData', async () => {
  // ... fetch logic
})
```

## Performance Improvements

### API Call Reduction
- **Before:** 720 calls/hour per user (5s interval)
- **After:** 240 calls/hour per user (15s interval)
- **Reduction:** 66% fewer API calls
- **Additional savings:** Pauses when tab hidden, caching reduces redundant calls

### Memory Management
- **Before:** Potential memory leaks from uncancelled requests
- **After:** All requests cancelled on unmount, no memory leaks

### User Experience
- **Before:** Continuous polling drains battery on mobile
- **After:** Smart polling pauses when not needed, better battery life

### Network Efficiency
- **Before:** Redundant concurrent requests
- **After:** Request deduplication, caching reduces network traffic

## Testing

Comprehensive test suite added: [HomeView.performance.test.ts](web/src/__tests__/views/HomeView.performance.test.ts)

Tests cover:
- ✅ 15-second polling interval
- ✅ Request cancellation on unmount
- ✅ Visibility API pause/resume
- ✅ Debounced auto-refresh toggle
- ✅ ShallowRef usage
- ✅ Resource cleanup
- ✅ AbortError handling
- ✅ Store caching (10s TTL)
- ✅ Request deduplication
- ✅ Force refresh bypass

## Migration Notes

### Breaking Changes
None - all changes are backward compatible

### Configuration
No configuration changes required. The optimizations are automatic.

### Monitoring
To enable performance monitoring in development:

```typescript
import { performanceMonitor } from '@/utils/performanceMonitor'

// Log performance summary
performanceMonitor.logSummary()

// Get average for specific operation
const avg = performanceMonitor.getAverage('HomeView:fetchData')
console.log(`Average fetch time: ${avg}ms`)
```

## Future Optimizations (Not Implemented)

### Low Priority
1. **WebSocket for Real-Time Updates** - Replace polling with WebSocket push
2. **Incremental Metrics Updates** - Only fetch new data points, not full history
3. **Virtual Scrolling** - If metrics history grows beyond 5 minutes
4. **Service Worker Caching** - Offline support and faster loads

### Considerations
- WebSocket requires backend changes
- Incremental updates need API modifications
- Virtual scrolling only needed if history window increases

## Verification

To verify optimizations are working:

1. **Check Network Tab:**
   - Requests should occur every 15 seconds (not 5)
   - No requests when tab is hidden
   - Concurrent requests should be deduplicated

2. **Check Console:**
   - No memory leak warnings
   - No AbortError logs
   - Performance warnings for operations >100ms

3. **Run Tests:**
   ```bash
   npm run test -- HomeView.performance.test.ts
   ```

## Performance Metrics

### Expected Results
- **Initial Load:** <500ms to first render
- **Polling Overhead:** <50ms per refresh cycle
- **Memory Usage:** Stable (no leaks)
- **Network Traffic:** 66% reduction

### Monitoring
Use browser DevTools Performance tab to verify:
- No long tasks (>50ms)
- Smooth 60fps rendering
- Minimal memory growth over time

## Conclusion

These optimizations significantly improve HomeView performance:
- **66% fewer API calls** reduces server load and network traffic
- **Smart polling** improves battery life on mobile devices
- **Request cancellation** prevents memory leaks
- **Caching and deduplication** reduce redundant work
- **Comprehensive cleanup** ensures no resource leaks

All changes are tested, documented, and backward compatible.
