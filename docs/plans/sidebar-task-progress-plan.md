# Sidebar Task Progress Widget

**Created**: 2026-06-19
**Status**: 🟢 Complete
**Priority**: Medium

---

## Background & Goals

Currently task status is only visible on the dedicated `/tasks` page. Users must navigate away from their current view to check task progress. This widget surfaces real-time task status directly in the sidebar, giving users at-a-glance visibility without leaving their current page.

## Technical Approach

- Add a `TaskProgress` Vue component at the top of the sidebar (above nav links)
- Reuse existing `useTaskPolling.js` singleton (already reactive, shared across components)
- No backend changes needed — all data comes from the existing task polling system
- Widget is purely a UI addition; no API contract changes

## Affected Files

| File | Action | Description |
|------|--------|-------------|
| `web/src/ui/components/TaskProgress.vue` | Create | New sidebar widget component |
| `web/src/ui/components/AppLayout.vue` | Modify | Import and render `TaskProgress` above nav links in sidebar |
| `web/src/ui/components/TaskBadge.vue` | — | Existing, no changes needed (reused for status display) |

## UI Design

### Layout (inside sidebar, above nav links)

```
┌─────────────────────────┐
│  ⚡ 任務進度              │  ← Header with running count badge
│  ████████░░░░  2/3       │  ← Progress bar (finished/total)
│                          │
│  🟢 執行中  2             │
│  ✅ 已完成  5             │
│  ❌ 錯誤    1             │
│                          │
│  ─────────────────────── │
│  最近任務:                │
│  • 1718... 執行中  🔄    │  ← Truncated task ID + TaskBadge
│  • 1718... 已完成  ✅    │
│  ─────────────────────── │
└─────────────────────────┘
```

### Behavior

- **No active tasks**: Widget shows "無執行中任務" (no active tasks) in muted style, minimal height
- **Has active tasks**: Shows progress bar, counts, and up to 3 most recent tasks
- **Click anywhere on widget**: Navigate to `/tasks` page
- **Progress bar**: `finished / total` where total = finished + running + error (excludes tasks still in `init`)
- **Mobile**: Widget appears in the collapsible sidebar (same as desktop), visible when sidebar is open

### Styling

- Tailwind CSS (v4), consistent with existing sidebar design
- Card-like container with subtle border/background to distinguish from nav links
- Progress bar uses `bg-blue-500` for finished, `bg-amber-400` for running portion
- Counts use colored dots (green/amber/red) matching `TaskBadge.vue` colors

## Tasks

| # | Task | Status | Sub Agent | Notes |
|---|------|--------|-----------|-------|
| 1 | Create `TaskProgress.vue` component | ✅ Complete | general | Reuse `useTaskPolling.js`, show counts + progress bar + recent tasks |
| 2 | Integrate into `AppLayout.vue` sidebar | ✅ Complete | general | Place above nav links, ensure mobile sidebar works |
| 3 | Verify build with `yarn build` | ✅ Complete | general | 通過。後續被 webui-layout-pagination-ai-badge-plan.md 取代為 header badge 模式 |

### Status Legend
- ⬜ Pending — not started
- 🔄 In Progress — sub agent working on it
- ✅ Complete — implemented and reviewed
- 🔴 Blocked — dependency or issue, needs resolution
- ⏭️ Skipped — not needed, with reason

## Testing Strategy

- No backend tests needed (no API changes)
- Manual verification: start dev server, trigger a migration, verify widget updates in real-time
- `yarn build` must pass

## Review Log

| Date | Reviewer | Findings | Action Taken |
|------|----------|----------|--------------|

## Open Questions

None.
