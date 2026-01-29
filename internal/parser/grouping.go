package parser

import (
	"sort"

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
			group.TotalApproved += stat.Approved
			group.TotalDenied += stat.Denied
			group.Children = append(group.Children, stat)
			if stat.LastSeen.After(group.LastSeen) {
				group.LastSeen = stat.LastSeen
			}
			if stat.ApprovedAt > group.ApprovedAt {
				group.ApprovedAt = stat.ApprovedAt
			}
		} else {
			groupMap[baseType] = &types.PermissionGroup{
				Type:          baseType,
				TotalCount:    stat.Count,
				TotalApproved: stat.Approved,
				TotalDenied:   stat.Denied,
				LastSeen:      stat.LastSeen,
				Children:      []types.PermissionStats{stat},
				Expanded:      false,
				ApprovedAt:    stat.ApprovedAt,
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
