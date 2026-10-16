package main

import (
	"flag"
	"fmt"
	"strings"

	"yandu/internal/inventory"
)

// flag.FlagSet stops at the first positional argument. Keep the usual flag
// validation while allowing `list blog --json` as well as `list --json blog`.
func parseInventoryFlags(fs *flag.FlagSet, args []string) ([]string, error) {
	var options, projects []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			projects = append(projects, args[i+1:]...)
			break
		}
		if arg == "-" || !strings.HasPrefix(arg, "-") {
			projects = append(projects, arg)
			continue
		}
		options = append(options, arg)
		name, _, hasValue := strings.Cut(strings.TrimLeft(arg, "-"), "=")
		option := fs.Lookup(name)
		if option == nil || hasValue {
			continue
		}
		if value, ok := option.Value.(interface{ IsBoolFlag() bool }); ok && value.IsBoolFlag() {
			continue
		}
		if i+1 < len(args) {
			i++
			options = append(options, args[i])
		}
	}
	return projects, fs.Parse(options)
}

func projectSelectors(args []string) ([]string, error) {
	var selectors []string
	seen := make(map[string]bool)
	for _, arg := range args {
		for _, value := range strings.Split(arg, ",") {
			value = strings.TrimSpace(value)
			if value == "" {
				return nil, fmt.Errorf("项目筛选项不能为空；用法：yandu list 项目ID或名称[,项目ID或名称...]")
			}
			if !seen[value] {
				selectors = append(selectors, value)
				seen[value] = true
			}
		}
	}
	return selectors, nil
}

// Select from a complete snapshot without modifying it. Shared disk capacity
// and reservations remain pool-wide; only the project list and disk set narrow.
func filterInventory(report inventory.Report, selectors []string) (inventory.Report, error) {
	if len(selectors) == 0 {
		return report, nil
	}
	filtered := report
	filtered.Projects = []inventory.Project{}
	filtered.Disks = []inventory.Disk{}
	filtered.Summary = inventory.Summary{}
	selected := make(map[string]bool)
	for _, selector := range selectors {
		// Stable IDs take precedence over another project's display name.
		id := ""
		for _, project := range report.Projects {
			if project.ID == selector {
				id = project.ID
				break
			}
		}
		if id == "" {
			var matches []string
			for _, project := range report.Projects {
				if project.Name == selector {
					matches = append(matches, project.ID)
				}
			}
			switch len(matches) {
			case 0:
				return filtered, fmt.Errorf("当前快照中未找到项目 %q；请使用完整项目 ID 或名称", selector)
			case 1:
				id = matches[0]
			default:
				return filtered, fmt.Errorf("项目名称 %q 对应多个项目，请改用 ID：%s", selector, strings.Join(matches, ", "))
			}
		}
		selected[id] = true
	}
	disks := make(map[string]bool)
	for _, project := range report.Projects {
		if selected[project.ID] {
			filtered.Projects = append(filtered.Projects, project)
			disks[project.DiskID] = true
		}
	}
	for _, disk := range report.Disks {
		if !disks[disk.ID] {
			continue
		}
		projectIDs := []string{}
		for _, id := range disk.ProjectIDs {
			if selected[id] {
				projectIDs = append(projectIDs, id)
			}
		}
		disk.ProjectIDs = projectIDs
		filtered.Disks = append(filtered.Disks, disk)
	}
	filtered.Summarize()
	return filtered, nil
}
