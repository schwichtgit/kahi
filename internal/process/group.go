package process

import (
	"fmt"
	"sort"
)

// BuildHomogeneousGroups creates implicit groups from program definitions.
// Each program creates a group with the same name containing all its instances.
func BuildHomogeneousGroups(programs map[string][]*Process) map[string]*Group {
	groups := make(map[string]*Group)

	for progName, procs := range programs {
		names := make([]string, 0, len(procs))
		priority := 999
		for _, p := range procs {
			names = append(names, p.Name())
			if p.Config().Priority < priority {
				priority = p.Config().Priority
			}
		}
		sort.Strings(names)

		groups[progName] = &Group{
			Name:      progName,
			Processes: names,
			Priority:  priority,
		}
	}

	return groups
}

// ValidateGroupNameCollisions checks for name collisions between
// program-generated implicit groups.
func ValidateGroupNameCollisions(groups map[string]*Group) error {
	seen := make(map[string]bool)
	for name := range groups {
		if seen[name] {
			return fmt.Errorf("group name collision: %q", name)
		}
		seen[name] = true
	}
	return nil
}

// MergeHeterogeneousGroups overlays explicit groups, suppressing
// implicit groups for member programs.
func MergeHeterogeneousGroups(implicit map[string]*Group, explicit map[string]*Group) map[string]*Group {
	result := make(map[string]*Group)

	// Track which programs are claimed by explicit groups.
	claimed := make(map[string]bool)
	for _, g := range explicit {
		for _, pName := range g.Processes {
			claimed[pName] = true
		}
		result[g.Name] = g
	}

	// Add implicit groups for unclaimed programs.
	for name, g := range implicit {
		if _, isExplicit := explicit[name]; isExplicit {
			continue // suppressed by explicit group
		}
		result[name] = g
	}

	return result
}
