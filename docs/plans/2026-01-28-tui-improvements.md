# TUI Improvements: Modal Fix & Drill-Down Feature

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Fix modal viewport positioning and add hierarchical drill-down for permissions (e.g., Bash → Bash(curl:*), Bash(git:*))

**Architecture:**
- Modal will use terminal dimensions and render inline (not overlaid) to avoid positioning bugs
- Permissions will be grouped hierarchically: parent type (Bash) with expandable children (Bash(curl:*))
- Tree view pattern from bubbletea components guide

**Tech Stack:** Bubbletea, Lipgloss, existing codebase

---

## Task 1: Fix Modal Positioning

The modal renders off-screen because it's using absolute positioning that doesn't account for actual terminal dimensions properly.

**Files:**
- Modify: `internal/view_apply.go`
- Modify: `internal/view.go`

**Step 1: Replace overlay approach with inline modal**

Change `renderWithModal` in `internal/view.go` to simply replace the main content with the modal (simpler, more reliable):

```go
// In view.go, replace renderWithModal function:
func (m Model) renderWithModal(background string) string {
    // Instead of overlay, just center the modal in available space
    modal := m.renderApplyModal()

    // Calculate centering
    modalLines := strings.Split(modal, "\n")
    modalHeight := len(modalLines)

    // Pad vertically to center
    topPadding := (m.height - modalHeight) / 2
    if topPadding < 0 {
        topPadding = 0
    }

    var result strings.Builder
    for i := 0; i < topPadding; i++ {
        result.WriteString("\n")
    }
    result.WriteString(modal)

    return result.String()
}
```

**Step 2: Make modal width responsive**

In `internal/view_apply.go`, use terminal width:

```go
func (m Model) renderApplyModal() string {
    perm := m.selectedPermission()
    if perm == nil {
        return ""
    }

    // Use 80% of terminal width, max 60
    modalWidth := m.width * 80 / 100
    if modalWidth > 60 {
        modalWidth = 60
    }
    if modalWidth < 40 {
        modalWidth = 40
    }

    // ... rest of function, but use modalWidth variable
```

**Step 3: Build and test**

Run: `go build -o bin/perms ./cmd/perms/ && ./bin/perms`
Expected: Modal appears centered, visible within viewport

**Step 4: Commit**

```bash
git add internal/view.go internal/view_apply.go
git commit -m "fix: center modal inline instead of overlay positioning"
```

---

## Task 2: Add Hierarchical Permission Data Structure

Group permissions by their base type (Bash, Read, etc.) with children for specific scopes.

**Files:**
- Modify: `internal/types/types.go`
- Modify: `internal/types.go`

**Step 1: Add hierarchical types**

In `internal/types/types.go`, add:

```go
// PermissionGroup represents a permission type with its children
type PermissionGroup struct {
    Type      string            // "Bash", "Read", etc.
    TotalCount int              // Sum of all children counts
    LastSeen  time.Time         // Most recent across all children
    Children  []PermissionStats // Individual permissions like Bash(curl:*)
    Expanded  bool              // UI state: is this group expanded?
    ApprovedAt ApprovalLevel    // Highest approval level among children
}
```

**Step 2: Add to Model**

In `internal/types.go`, update Model:

```go
type Model struct {
    // ... existing fields ...

    // Hierarchical view
    permissionGroups []types.PermissionGroup
    groupCursor      int  // Which group is selected
    childCursor      int  // Which child within expanded group (-1 if on group)
}
```

**Step 3: Build to verify**

Run: `go build ./...`
Expected: Compiles without errors

**Step 4: Commit**

```bash
git add internal/types/types.go internal/types.go
git commit -m "feat: add hierarchical permission group data structures"
```

---

## Task 3: Build Permission Grouping Logic

Parse flat permissions into hierarchical groups.

**Files:**
- Create: `internal/parser/grouping.go`
- Modify: `internal/model.go`

**Step 1: Create grouping logic**

Create `internal/parser/grouping.go`:

```go
package parser

import (
    "sort"
    "time"

    "github.com/b-open-io/claude-perms/internal/types"
)

// GroupPermissions converts flat permission stats into hierarchical groups
func GroupPermissions(stats []types.PermissionStats) []types.PermissionGroup {
    // Map to group by base type
    groupMap := make(map[string]*types.PermissionGroup)

    for _, stat := range stats {
        baseType := stat.Permission.Type

        if group, exists := groupMap[baseType]; exists {
            group.TotalCount += stat.Count
            group.Children = append(group.Children, stat)
            if stat.LastSeen.After(group.LastSeen) {
                group.LastSeen = stat.LastSeen
            }
            if stat.ApprovedAt > group.ApprovedAt {
                group.ApprovedAt = stat.ApprovedAt
            }
        } else {
            groupMap[baseType] = &types.PermissionGroup{
                Type:       baseType,
                TotalCount: stat.Count,
                LastSeen:   stat.LastSeen,
                Children:   []types.PermissionStats{stat},
                Expanded:   false,
                ApprovedAt: stat.ApprovedAt,
            }
        }
    }

    // Convert map to slice and sort by count
    groups := make([]types.PermissionGroup, 0, len(groupMap))
    for _, g := range groupMap {
        // Sort children by count
        sort.Slice(g.Children, func(i, j int) bool {
            return g.Children[i].Count > g.Children[j].Count
        })
        groups = append(groups, *g)
    }

    // Sort groups by total count
    sort.Slice(groups, func(i, j int) bool {
        return groups[i].TotalCount > groups[j].TotalCount
    })

    return groups
}
```

**Step 2: Integrate into model loading**

In `internal/model.go`, update `LoadData` and the dataLoadedMsg handler:

```go
// In dataLoadedMsg struct, add:
permissionGroups []types.PermissionGroup

// In LoadData function, after loading permissions:
groups := parser.GroupPermissions(permissions)

// Return with groups:
return dataLoadedMsg{
    permissions:      permissions,
    permissionGroups: groups,
    // ... rest
}

// In Update handler for dataLoadedMsg:
m.permissionGroups = msg.permissionGroups
```

**Step 3: Build and test**

Run: `go build ./... && go run ./cmd/test-parser/`
Expected: Compiles, test parser still works

**Step 4: Commit**

```bash
git add internal/parser/grouping.go internal/model.go
git commit -m "feat: add permission grouping logic"
```

---

## Task 4: Update Frequency View for Tree Display

Render hierarchical tree with expand/collapse.

**Files:**
- Modify: `internal/view_frequency.go`
- Modify: `internal/update_keyboard.go`

**Step 1: Update view to show tree**

Replace `renderFrequencyView` in `internal/view_frequency.go`:

```go
func (m Model) renderFrequencyView() string {
    _, contentHeight := m.calculateLayout()

    var lines []string

    // Header
    lines = append(lines, renderFrequencyHeader())
    lines = append(lines, styles.ListHeader.Render(strings.Repeat("─", m.width-4)))

    // Track current line for cursor
    lineIndex := 0

    for gi, group := range m.permissionGroups {
        if len(lines) >= contentHeight {
            break
        }

        // Render group header
        isGroupSelected := gi == m.groupCursor && m.childCursor == -1
        expandChar := "▶"
        if group.Expanded {
            expandChar = "▼"
        }

        groupLine := renderGroupRow(group, expandChar, isGroupSelected, m.width)
        lines = append(lines, groupLine)
        lineIndex++

        // Render children if expanded
        if group.Expanded {
            for ci, child := range group.Children {
                if len(lines) >= contentHeight {
                    break
                }
                isChildSelected := gi == m.groupCursor && ci == m.childCursor
                childLine := renderChildRow(child, isChildSelected, m.width)
                lines = append(lines, childLine)
                lineIndex++
            }
        }
    }

    // Pad remaining lines
    for len(lines) < contentHeight {
        lines = append(lines, "")
    }

    return strings.Join(lines[:contentHeight], "\n") + "\n"
}

func renderGroupRow(g types.PermissionGroup, expandChar string, selected bool, width int) string {
    count := fmt.Sprintf("%d", g.TotalCount)
    countStyled := styles.ColCount.Render(count)

    name := fmt.Sprintf("%s %s", expandChar, g.Type)
    if len(g.Children) > 1 {
        name += fmt.Sprintf(" (%d variants)", len(g.Children))
    }
    nameStyled := styles.ColPerm.Render(truncateString(name, 35))

    timeStyled := styles.ColTime.Render(formatRelativeTime(g.LastSeen))

    var statusStyled string
    if g.ApprovedAt > types.NotApproved {
        statusStyled = styles.StatusApproved.Render(g.ApprovedAt.String())
    } else {
        statusStyled = styles.StatusPending.Render("○")
    }

    row := fmt.Sprintf("%s  %s  %s  %s", countStyled, nameStyled, timeStyled, statusStyled)

    if selected {
        return styles.ListItemSelected.Render("> " + row)
    }
    return styles.ListItem.Render(row)
}

func renderChildRow(p types.PermissionStats, selected bool, width int) string {
    count := fmt.Sprintf("%d", p.Count)
    countStyled := styles.ColCount.Render(count)

    // Indent child with scope only
    name := "    " + p.Permission.Raw
    nameStyled := styles.ColPerm.Render(truncateString(name, 35))

    timeStyled := styles.ColTime.Render(formatRelativeTime(p.LastSeen))

    var statusStyled string
    if p.ApprovedAt > types.NotApproved {
        statusStyled = styles.StatusApproved.Render(p.ApprovedAt.String())
    } else {
        statusStyled = styles.StatusPending.Render("○")
    }

    row := fmt.Sprintf("%s  %s  %s  %s", countStyled, nameStyled, timeStyled, statusStyled)

    if selected {
        return styles.ListItemSelected.Render("> " + row)
    }
    return styles.ListItem.Render(row)
}
```

**Step 2: Update keyboard navigation**

In `internal/update_keyboard.go`, update navigation:

```go
case "j", "down":
    m.navigateDown()
    return m, nil

case "k", "up":
    m.navigateUp()
    return m, nil

case "enter", " ":
    // If on a group, toggle expand
    if m.childCursor == -1 && m.groupCursor < len(m.permissionGroups) {
        m.permissionGroups[m.groupCursor].Expanded = !m.permissionGroups[m.groupCursor].Expanded
    }
    // If on a child, show modal
    if m.childCursor >= 0 {
        m.showApplyModal = true
    }
    return m, nil
```

**Step 3: Add navigation helpers to model.go**

```go
func (m *Model) navigateDown() {
    if len(m.permissionGroups) == 0 {
        return
    }

    group := &m.permissionGroups[m.groupCursor]

    if m.childCursor == -1 {
        // On group header
        if group.Expanded && len(group.Children) > 0 {
            // Move into children
            m.childCursor = 0
        } else {
            // Move to next group
            if m.groupCursor < len(m.permissionGroups)-1 {
                m.groupCursor++
            }
        }
    } else {
        // In children
        if m.childCursor < len(group.Children)-1 {
            m.childCursor++
        } else {
            // Move to next group
            if m.groupCursor < len(m.permissionGroups)-1 {
                m.groupCursor++
                m.childCursor = -1
            }
        }
    }
}

func (m *Model) navigateUp() {
    if len(m.permissionGroups) == 0 {
        return
    }

    if m.childCursor == -1 {
        // On group header
        if m.groupCursor > 0 {
            m.groupCursor--
            prevGroup := &m.permissionGroups[m.groupCursor]
            if prevGroup.Expanded && len(prevGroup.Children) > 0 {
                m.childCursor = len(prevGroup.Children) - 1
            }
        }
    } else {
        // In children
        if m.childCursor > 0 {
            m.childCursor--
        } else {
            m.childCursor = -1 // Back to group header
        }
    }
}
```

**Step 4: Build and test**

Run: `go build -o bin/perms ./cmd/perms/`
Expected: Compiles

**Step 5: Commit**

```bash
git add internal/view_frequency.go internal/update_keyboard.go internal/model.go
git commit -m "feat: add tree view with expand/collapse for permission groups"
```

---

## Task 5: Update Selected Permission for Modal

The modal needs to know which permission is selected (group or child).

**Files:**
- Modify: `internal/model.go`

**Step 1: Update selectedPermission method**

```go
func (m Model) selectedPermission() *types.PermissionStats {
    if len(m.permissionGroups) == 0 {
        return nil
    }
    if m.groupCursor >= len(m.permissionGroups) {
        return nil
    }

    group := &m.permissionGroups[m.groupCursor]

    if m.childCursor >= 0 && m.childCursor < len(group.Children) {
        // Return specific child
        return &group.Children[m.childCursor]
    }

    // Return first child of group (or nil if no children)
    if len(group.Children) > 0 {
        return &group.Children[0]
    }
    return nil
}
```

**Step 2: Build and test interactively**

Run: `go build -o bin/perms ./cmd/perms/ && ./bin/perms`
Expected: Can navigate with j/k, expand with Enter/Space, see children

**Step 3: Commit**

```bash
git add internal/model.go
git commit -m "feat: update selectedPermission for hierarchical navigation"
```

---

## Task 6: Initialize Model with Group Cursors

**Files:**
- Modify: `internal/model.go`

**Step 1: Update NewModel**

```go
func NewModel() Model {
    ti := textinput.New()
    ti.Placeholder = "Filter..."
    ti.CharLimit = 50

    cwd, _ := os.Getwd()

    return Model{
        activeView:       ViewFrequency,
        showApplyModal:   false,
        permissions:      nil,
        permissionGroups: nil,
        agents:           nil,
        skills:           nil,
        userApproved:     nil,
        projectApproved:  nil,
        projectPath:      cwd,
        cursor:           0,
        groupCursor:      0,
        childCursor:      -1, // Start on group, not child
        filterInput:      ti,
        filtering:        false,
        filteredIndices:  nil,
        width:            80,
        height:           24,
        err:              nil,
    }
}
```

**Step 2: Build final version**

Run: `go build -o bin/perms ./cmd/perms/`

**Step 3: Test full flow**

1. Run `./bin/perms`
2. Navigate with j/k
3. Press Enter on Bash to expand
4. See Bash(curl:*), Bash(git:*), etc.
5. Navigate into children
6. Press Enter on a child to see modal

**Step 4: Final commit**

```bash
git add internal/model.go
git commit -m "feat: complete hierarchical permission viewer with expand/collapse"
```

---

## Summary

After completing all tasks:

1. ✅ Modal renders centered and visible
2. ✅ Permissions grouped by type (Bash, Read, Edit, etc.)
3. ✅ Expand/collapse groups with Enter/Space
4. ✅ Navigate into children with j/k
5. ✅ Modal shows selected child permission
