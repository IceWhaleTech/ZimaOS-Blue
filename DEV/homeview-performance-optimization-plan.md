# HomeView Performance Optimization Plan

## Current Performance Issues

### Critical Issues
1. **Aggressive Polling (5s interval)** - 720 API calls/hour per user
2. **Multiple Independent Polling** - Uncoordinated refresh cycles
3. **No Request Cancellation** - Memory leaks on unmount
4. **No Debouncing** - Potential API flooding

### Moderate Issues
5. **Metrics History Processing** - Recalculation on every render
6. **No Data Caching** - Redundant data transfer
7. **Synchronous localStorage** - Blocks main thread

## Optimization Strategy

### Phase 1: Polling Optimization (HIGH PRIORITY)
- [ ] Increase polling interval from 5s to 15s (reduce API calls by 66%)
- [ ] Add request cancellation with AbortController
- [ ] Implement visibility API to pause polling when tab is hidden
- [ ] Add debouncing to autoRefresh toggle

### Phase 2: Data Management (MEDIUM PRIORITY)
- [ ] Implement incremental metrics updates
- [ ] Add client-side caching with TTL
- [ ] Debounce localStorage writes
- [ ] Use shallowRef for large arrays

### Phase 3: Smart Refresh (MEDIUM PRIORITY)
- [ ] Create centralized polling composable
- [ ] Implement adaptive polling (slow down when idle)
- [ ] Add request deduplication
- [ ] Coordinate all card refresh cycles

### Phase 4: Monitoring (LOW PRIORITY)
- [ ] Add performance metrics tracking
- [ ] Implement error boundaries
- [ ] Add loading states optimization

## Implementation Order

1. Read current HomeView.vue implementation
2. Optimize polling interval and add visibility API
3. Add request cancellation and debouncing
4. Implement data caching and optimization
5. Create centralized polling composable
6. Test and verify improvements

## Success Metrics

- API calls reduced from 720/hour to 240/hour (66% reduction)
- No memory leaks on component unmount
- Smooth UI with no jank during updates
- Proper cleanup of all intervals and listeners
