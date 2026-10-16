package main

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/mattn/go-runewidth"
	"golang.org/x/term"
	"yandu/internal/inventory"
)

// Terminal cells, rather than bytes or rune counts, determine column alignment.
// Ambiguous symbols use one cell, matching modern UTF-8 SSH terminals.
var cells = &runewidth.Condition{EastAsianWidth: false, StrictEmojiNeutral: true}

func cleanCell(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		if unicode.Is(unicode.Cf, r) && r != '\u200d' && r != '\u200c' {
			return -1
		}
		return r
	}, s)
}

func humanBytes(n *int64) string {
	if n == nil {
		return "未知"
	}
	if *n < 1024 {
		return fmt.Sprintf("%d B", *n)
	}
	x := float64(*n)
	units := []string{"B", "KiB", "MiB", "GiB", "TiB", "PiB"}
	i := 0
	for x >= 1024 && i < len(units)-1 {
		x /= 1024
		i++
	}
	return fmt.Sprintf("%.2f %s", x, units[i])
}

func inventoryWidth(out io.Writer) int {
	if file, ok := out.(interface{ Fd() uintptr }); ok {
		if width, _, err := term.GetSize(int(file.Fd())); err == nil && width > 0 {
			return max(24, min(width, 180))
		}
	}
	if width, err := strconv.Atoi(os.Getenv("COLUMNS")); err == nil && width > 0 {
		return max(24, min(width, 180))
	}
	return 120
}

type inventoryPrinter struct {
	out   io.Writer
	width int
	err   error
}

func (p *inventoryPrinter) line(s string) {
	if p.err == nil {
		_, p.err = fmt.Fprintln(p.out, s)
	}
}

// Continuation lines align with the value, retaining complete paths and IDs.
func (p *inventoryPrinter) text(prefix, value string) {
	indent := strings.Repeat(" ", cells.StringWidth(prefix))
	for i, line := range strings.Split(cells.Wrap(cleanCell(value), max(2, p.width-cells.StringWidth(prefix))), "\n") {
		if i == 0 {
			p.line(prefix + line)
		} else {
			p.line(indent + line)
		}
	}
}

type inventoryColumn struct {
	title      string
	min, max   int
	rightAlign bool
}

type inventoryDetail struct {
	label, value string
	rightAlign   bool
}

type inventoryRow struct {
	values  []string
	details []inventoryDetail
}

func padCell(s string, width int, right bool) string {
	padding := strings.Repeat(" ", max(0, width-cells.StringWidth(s)))
	if right {
		return padding + s
	}
	return s + padding
}

func wrapCell(value string, width int) []string {
	var lines []string
	for _, paragraph := range strings.Split(value, "\n") {
		lines = append(lines, strings.Split(cells.Wrap(cleanCell(paragraph), width), "\n")...)
	}
	return lines
}

func detailLines(detail inventoryDetail, width int) []string {
	label := cleanCell(detail.label)
	labelWidth := cells.StringWidth(label)
	if detail.rightAlign && labelWidth+cells.StringWidth(cleanCell(detail.value)) <= width {
		return []string{label + padCell(cleanCell(detail.value), width-labelWidth, true)}
	}
	lines := wrapCell(cleanCell(detail.value), max(2, width-labelWidth))
	for i := range lines {
		if i == 0 {
			lines[i] = label + lines[i]
		} else {
			lines[i] = strings.Repeat(" ", labelWidth) + lines[i]
		}
	}
	return lines
}

// Every record includes its own details inside the border, so a wrapped path
// remains visibly attached to the project or disk it describes.
// Returns false when the columns cannot fit; the caller then uses bordered cards.
func (p *inventoryPrinter) table(columns []inventoryColumn, rows []inventoryRow) bool {
	widths := make([]int, len(columns))
	minimums := make([]int, len(columns))
	total := 3*len(columns) + 1 // One space on each side of every cell, plus borders.
	for i, column := range columns {
		minimums[i] = max(column.min, cells.StringWidth(column.title))
		widths[i] = minimums[i]
		for _, row := range rows {
			for _, line := range strings.Split(row.values[i], "\n") {
				width := cells.StringWidth(cleanCell(line))
				widths[i] = max(widths[i], min(column.max, width))
				if column.rightAlign {
					// Keep each number and its unit together, even at narrow widths.
					minimums[i] = max(minimums[i], width)
					widths[i] = max(widths[i], minimums[i])
				}
			}
		}
		total += widths[i]
	}
	for total > p.width {
		widest := -1
		for i := range columns {
			if widths[i] > minimums[i] && (widest == -1 || widths[i] > widths[widest]) {
				widest = i
			}
		}
		if widest == -1 {
			return false
		}
		widths[widest]--
		total--
	}
	// A long path may need more room than the compact summary columns. Use
	// available terminal space before wrapping the full-width detail cell.
	preferred := total
	for _, row := range rows {
		for _, detail := range row.details {
			preferred = max(preferred, cells.StringWidth(cleanCell(detail.label+detail.value))+4)
		}
	}
	if extra := min(preferred, p.width) - total; extra > 0 {
		widths[0] += extra
		total += extra
	}
	border := func(left, joint, right string) {
		parts := make([]string, len(widths))
		for i, width := range widths {
			parts[i] = strings.Repeat("─", width+2)
		}
		p.line(left + strings.Join(parts, joint) + right)
	}
	printRow := func(row []string) {
		lines := make([][]string, len(columns))
		height := 1
		for i, value := range row {
			lines[i] = wrapCell(value, widths[i])
			height = max(height, len(lines[i]))
		}
		for line := 0; line < height; line++ {
			parts := make([]string, len(columns))
			for i, column := range columns {
				value := ""
				if line < len(lines[i]) {
					value = lines[i][line]
				}
				parts[i] = padCell(value, widths[i], column.rightAlign)
			}
			p.line("│ " + strings.Join(parts, " │ ") + " │")
		}
	}
	headers := make([]string, len(columns))
	for i, column := range columns {
		headers[i] = column.title
	}
	border("┌", "┬", "┐")
	printRow(headers)
	border("├", "┼", "┤")
	for i, row := range rows {
		printRow(row.values)
		if len(row.details) > 0 {
			border("├", "┴", "┤")
			for _, detail := range row.details {
				for _, line := range detailLines(detail, total-4) {
					p.line("│ " + padCell(line, total-4, false) + " │")
				}
			}
			if i < len(rows)-1 {
				border("├", "┬", "┤")
			} else {
				p.line("└" + strings.Repeat("─", total-2) + "┘")
			}
		} else if i < len(rows)-1 {
			border("├", "┼", "┤")
		} else {
			border("└", "┴", "┘")
		}
	}
	return true
}

func (p *inventoryPrinter) card(title string, details []inventoryDetail) {
	p.line("┌" + strings.Repeat("─", p.width-2) + "┐")
	for _, line := range wrapCell(title, p.width-4) {
		p.line("│ " + padCell(line, p.width-4, false) + " │")
	}
	p.line("├" + strings.Repeat("─", p.width-2) + "┤")
	for _, detail := range details {
		for _, line := range detailLines(detail, p.width-4) {
			p.line("│ " + padCell(line, p.width-4, false) + " │")
		}
	}
	p.line("└" + strings.Repeat("─", p.width-2) + "┘")
}

func displayState(value string) string {
	labels := map[string]string{
		"": "—", "active": "已生效", "draft": "未发布", "disabled": "已停用",
		"pending": "待应用", "applying": "应用中", "disabled_pending": "待同步",
		"waiting_tunnel": "等待隧道", "waiting_ingress": "等待入口", "unverified": "待验证",
		"local_invalid": "配置无效", "recovering": "恢复中", "cloud_blocked": "已封禁",
		"enabled": "启用", "uninitialized": "未初始化", "quarantined": "已隔离",
	}
	if label, ok := labels[value]; ok {
		return label
	}
	return value
}

func displayMode(value string) string {
	switch value {
	case "application_storage":
		return "存储"
	case "readonly_mapping", "":
		return "只读映射"
	default:
		return value
	}
}

func capacityDetails(labels, values []string) []inventoryDetail {
	var details []inventoryDetail
	for i, label := range labels {
		details = append(details, inventoryDetail{label + "：", values[i], true})
	}
	return details
}

func printInventory(out io.Writer, r inventory.Result, disksOnly bool) error {
	return printInventoryWidth(out, r, disksOnly, inventoryWidth(out))
}

func printInventoryWidth(out io.Writer, r inventory.Result, disksOnly bool, width int) error {
	p := &inventoryPrinter{out: out, width: max(24, width)}
	title := "檐渡 · 项目与容量"
	if disksOnly {
		title = "檐渡 · 磁盘容量"
	}
	p.line(title)
	state := "在线"
	if !r.Online {
		state = "不可达"
	}
	if r.Stale {
		state += "（历史快照）"
	}
	p.text("设备：", r.DeviceID+"  平台："+r.Platform+"  状态："+state)
	if !r.SampledAt.IsZero() {
		p.text("采样：", r.SampledAt.Format(time.RFC3339)+fmt.Sprintf("  快照年龄：%ds", r.SnapshotAgeSeconds))
	}
	if r.Stale {
		p.text("注意：", "下列为历史快照，容量不可作为当前写入依据。")
	}
	if r.Error != "" {
		p.text("查询：", r.Error)
	}
	if !r.Online && !r.Stale {
		p.line("没有可用的设备快照。")
		return p.err
	}
	diskLabels := map[string]string{}
	for i, disk := range r.Disks {
		diskLabels[disk.ID] = fmt.Sprintf("D%d", i+1)
	}
	if !disksOnly {
		p.line("")
		p.line(fmt.Sprintf("项目（%d）", len(r.Projects)))
		rows := []inventoryRow{}
		for _, project := range r.Projects {
			disk := diskLabels[project.DiskID]
			if disk == "" {
				disk = "未知"
			}
			identity := cleanCell(project.Name)
			if identity == "" || identity == project.ID {
				identity = cleanCell(project.ID)
			} else {
				identity += "\n" + cleanCell(project.ID)
			}
			state := "配置：" + cleanCell(displayState(project.State))
			if project.StorageState != "" {
				state += "\n存储：" + cleanCell(displayState(project.StorageState))
			}
			details := []inventoryDetail{
				{"模式：", displayMode(project.Mode) + "  ·  磁盘：" + disk, false},
				{"目录：", project.RootPath, false},
			}
			if project.Error != "" {
				details = append(details, inventoryDetail{"错误：", project.Error, false})
			}
			rows = append(rows, inventoryRow{
				[]string{identity, state, humanBytes(project.QuotaBytes), humanBytes(project.UsedBytes), humanBytes(project.ReservedBytes), humanBytes(project.RemainingQuotaBytes)},
				details,
			})
		}
		columns := []inventoryColumn{{"项目 / ID", 10, 32, false}, {"状态", 12, 18, false}, {"配额", 9, 12, true}, {"已用", 9, 12, true}, {"预留", 8, 12, true}, {"剩余额度", 9, 12, true}}
		if len(rows) == 0 {
			p.line("暂无项目。请在本机管理页创建项目。")
		} else if !p.table(columns, rows) {
			for i, row := range rows {
				if i > 0 {
					p.line("")
				}
				project := r.Projects[i]
				details := []inventoryDetail{{"ID：", project.ID, false}, {"配置：", displayState(project.State), false}}
				if project.StorageState != "" {
					details = append(details, inventoryDetail{"存储：", displayState(project.StorageState), false})
				}
				details = append(details, capacityDetails([]string{"配额", "已用", "预留", "剩余额度"}, row.values[2:])...)
				details = append(details, row.details...)
				title := cleanCell(project.Name)
				if title == "" {
					title = cleanCell(project.ID)
				}
				p.card(title, details)
			}
		}
	}
	p.line("")
	p.line(fmt.Sprintf("磁盘 / 共享容器（%d）", len(r.Disks)))
	rows := []inventoryRow{}
	for _, disk := range r.Disks {
		state := "可查询"
		if disk.Error != "" {
			state = "异常"
		}
		details := []inventoryDetail{{"标识：", disk.ID, false}, {"挂载：", strings.Join(disk.MountPoints, ", "), false}, {"项目：", strings.Join(disk.ProjectIDs, ", "), false}}
		if disk.Error != "" {
			details = append(details, inventoryDetail{"错误：", disk.Error, false})
		}
		rows = append(rows, inventoryRow{[]string{diskLabels[disk.ID], disk.Filesystem, humanBytes(disk.TotalBytes), humanBytes(disk.FreeBytes), humanBytes(inventory.Bytes(disk.ReservedBytes)), humanBytes(disk.AvailableAfterReservationsBytes), state}, details})
	}
	columns := []inventoryColumn{{"磁盘", 4, 6, false}, {"文件系统", 8, 12, false}, {"总容量", 10, 12, true}, {"空闲", 10, 12, true}, {"预留", 9, 12, true}, {"可用空间", 10, 12, true}, {"状态", 6, 8, false}}
	if len(rows) == 0 {
		p.line("暂无可查询的受管磁盘。")
	} else if !p.table(columns, rows) {
		for i, disk := range r.Disks {
			if i > 0 {
				p.line("")
			}
			details := []inventoryDetail{{"文件系统：", disk.Filesystem, false}, {"状态：", rows[i].values[6], false}}
			details = append(details, capacityDetails([]string{"总容量", "空闲", "预留", "可用空间"}, rows[i].values[2:6])...)
			details = append(details, rows[i].details...)
			p.card(diskLabels[disk.ID], details)
		}
	}
	p.line("")
	p.text("", fmt.Sprintf("合计：%d 项目，%d 个受管磁盘/容器", r.Summary.ProjectCount, r.Summary.DiskCount))
	capacity := []string{humanBytes(r.Summary.TotalBytes), humanBytes(r.Summary.FreeBytes), humanBytes(r.Summary.AvailableAfterReservationsBytes)}
	if !p.table([]inventoryColumn{{"总容量", 9, 12, true}, {"空闲", 9, 12, true}, {"可用空间", 9, 12, true}}, []inventoryRow{{values: capacity}}) {
		p.card("容量合计", capacityDetails([]string{"总容量", "空闲", "可用空间"}, capacity))
	}
	p.line("")
	p.text("说明：", "可用空间已扣除预留及安全余量，不等于可上传文件大小。")
	p.text("      ", "仅统计项目所在磁盘，共享空间只计一次；未知项会使合计显示未知。")
	p.text("      ", "只读映射不扫描文件占用。")
	return p.err
}
