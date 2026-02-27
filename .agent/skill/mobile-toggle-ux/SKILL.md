# Mobile Toggle UX Optimization Skill

## Purpose
Optimize crowded mobile toggle UI into clear, touch-friendly grouped controls, with a clearly visible `Auto` mode.

## When to Use
- Mobile settings page has too many toggles in collapsed sections
- Users cannot quickly identify whether current mode is automatic
- Touch accuracy and mode comprehension are both poor

## Core Goals
1. Reduce visual clutter on first screen
2. Make `Auto (Recommended)` obvious at a glance
3. Improve one-handed touch interaction and hit rate
4. Clarify behavioral difference between auto and manual modes

## Information Architecture
1. Two-layer display:
- Layer 1: group summary only (title + current state)
- Layer 2: expanded controls with full options
2. Each group should include:
- Group title
- Current value summary
- Explicit chevron expand/collapse trigger
- Expanded segmented control and helper text

## Component Rules (Mobile)
### OptionGroupCard
- Border radius: `12px`
- Padding: `12px`
- Gap between groups: `10px`
- Background: `--surface-1`
- Border: `1px solid --border-subtle`

### Header Area
- Title: `15px semibold`
- Current value text: `13px`
- Current value color:
- `--text-secondary` for normal
- `--accent-700` when current is auto
- Expand button hit area: minimum `40x40`

### Options Area (Expanded)
- Use `SegmentedControl` instead of scattered independent toggles
- Option minimum height: `36px`
- Horizontal option padding: `12px`
- Selected state: `--accent-600` + white text
- Unselected state: `--surface-2` + `--text-primary`

## Auto Mode Emphasis (Required)
1. Label must be:
- `自动（推荐）` (or English locale `Auto (Recommended)`)
2. Visual emphasis:
- `AUTO` badge on the left of label
- Always place auto option first
- When auto selected, add subtle group-level highlight (example `--accent-50`)
3. Helper text:
- Auto: `根据网络质量与负载动态选择`
- Manual: `已固定为手动模式，不会自动切换`

## Interaction Rules
1. Collapsed default should show only:
- `当前：自动（推荐）` or `当前：手动-XX`
2. Feedback:
- Switching to auto: `已切换为自动模式`
- Leaving auto for manual: `已关闭自动选择`
3. Error prevention:
- Debounce repeated switching within `300ms`
- Entire option row must be clickable, not text-only

## Suggested State Model
```ts
type ModeOption = {
  key: string;          // auto | fast | stable...
  label: string;        // 自动（推荐）
  isRecommended?: boolean;
  helperText?: string;
};

type OptionGroup = {
  id: string;           // network-mode
  title: string;        // 连接模式
  selectedKey: string;  // auto
  collapsed: boolean;   // true by default on mobile
  options: ModeOption[];
};
```

## Acceptance Criteria
1. No multi-row crowded toggle wall appears on first mobile screen
2. User can identify whether current mode is auto without expanding
3. Auto style and label are consistent across all groups
4. Touch target quality:
- Option height `>=36`
- Tap target `>=40`
