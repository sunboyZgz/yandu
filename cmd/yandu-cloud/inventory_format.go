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

func padCell(s string, width int, right bool) string {
	padding := strings.Repeat(" ", max(0, width-cells.StringWidth(s)))
	if right {
		return padding + s
	}
	return s + padding
}

// Returns false when the columns cannot fit; the caller then uses details.
func (p *inventoryPrinter) table(columns []inventoryColumn, rows [][]string) bool {
	widths := make([]int, len(columns))
	total := 2 * (len(columns) - 1)
	for i, column := range columns {
		widths[i] = max(column.min, cells.StringWidth(column.title))
		for _, row := range rows {
			widths[i] = max(widths[i], min(column.max, cells.StringWidth(cleanCell(row[i]))))
		}
		total += widths[i]
	}
	for total > p.width {
		widest := -1
		for i, column := range columns {
			if widths[i] > max(column.min, cells.StringWidth(column.title)) && (widest == -1 || widths[i] > widths[widest]) {
				widest = i
			}
		}
		if widest == -1 {
			return false
		}
		widths[widest]--
		total--
	}
	printRow := func(row []string) {
		lines := make([][]string, len(columns))
		height := 1
		for i, value := range row {
			lines[i] = strings.Split(cells.Wrap(cleanCell(value), widths[i]), "\n")
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
			p.line(strings.TrimRight(strings.Join(parts, "  "), " "))
		}
	}
	headers := make([]string, len(columns))
	for i, column := range columns {
		headers[i] = column.title
	}
	printRow(headers)
	p.line(strings.Repeat("─", total))
	for _, row := range rows {
		printRow(row)
	}
	return true
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

func (p *inventoryPrinter) metrics(labels, values []string) {
	perLine := 1
	if p.width >= 56 {
		perLine = 2
	}
	for start := 0; start < len(labels); start += perLine {
		parts := []string{}
		for i := start; i < min(start+perLine, len(labels)); i++ {
			parts = append(parts, padCell(labels[i], 8, false)+"  "+padCell(values[i], 12, true))
		}
		p.line("  " + strings.Join(parts, "    "))
	}
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
		rows := [][]string{}
		for _, project := range r.Projects {
			disk := diskLabels[project.DiskID]
			if disk == "" {
				disk = "未知"
			}
			rows = append(rows, []string{project.ID, project.Name, displayMode(project.Mode), displayState(project.State), displayState(project.StorageState), disk, humanBytes(project.QuotaBytes), humanBytes(project.UsedBytes), humanBytes(project.ReservedBytes), humanBytes(project.RemainingQuotaBytes)})
		}
		columns := []inventoryColumn{{"项目 ID", 10, 24, false}, {"名称", 8, 20, false}, {"模式", 8, 12, false}, {"配置状态", 8, 16, false}, {"存储状态", 8, 12, false}, {"磁盘", 4, 6, false}, {"配额", 9, 12, true}, {"已用", 9, 12, true}, {"预留", 8, 12, true}, {"剩余额度", 9, 12, true}}
		if len(rows) == 0 {
			p.line("暂无项目。请在本机管理页创建项目。")
		} else if p.table(columns, rows) {
			p.line("")
			p.line("项目明细")
			for _, project := range r.Projects {
				p.text("  ", project.ID+" · "+project.Name)
				p.text("    目录：", project.RootPath)
				if project.Error != "" {
					p.text("    错误：", project.Error)
				}
			}
		} else {
			for i, row := range rows {
				if i > 0 {
					p.line("")
				}
				p.text(fmt.Sprintf("%d. ", i+1), row[1])
				p.text("  ID：", row[0])
				p.text("  状态：", row[2]+" / "+row[3]+"  存储："+row[4]+"  磁盘："+row[5])
				p.metrics([]string{"配额", "已用", "预留", "剩余额度"}, row[6:])
				p.text("  目录：", r.Projects[i].RootPath)
				if r.Projects[i].Error != "" {
					p.text("  错误：", r.Projects[i].Error)
				}
			}
		}
	}
	p.line("")
	p.line(fmt.Sprintf("磁盘 / 共享容器（%d）", len(r.Disks)))
	rows := [][]string{}
	for _, disk := range r.Disks {
		state := "可查询"
		if disk.Error != "" {
			state = "异常"
		}
		rows = append(rows, []string{diskLabels[disk.ID], disk.Filesystem, humanBytes(disk.TotalBytes), humanBytes(disk.FreeBytes), humanBytes(inventory.Bytes(disk.ReservedBytes)), humanBytes(disk.AvailableAfterReservationsBytes), state})
	}
	columns := []inventoryColumn{{"磁盘", 4, 6, false}, {"文件系统", 8, 12, false}, {"总容量", 10, 12, true}, {"空闲", 10, 12, true}, {"预留", 9, 12, true}, {"可用空间", 10, 12, true}, {"状态", 6, 8, false}}
	if len(rows) == 0 {
		p.line("暂无可查询的受管磁盘。")
	} else {
		table := p.table(columns, rows)
		for i, disk := range r.Disks {
			p.line("")
			if table {
				p.text("  "+diskLabels[disk.ID]+" 标识：", disk.ID)
			} else {
				p.text(diskLabels[disk.ID]+" · ", disk.Filesystem+" / "+rows[i][6])
				p.metrics([]string{"总容量", "空闲", "预留", "可用空间"}, rows[i][2:6])
				p.text("  标识：", disk.ID)
			}
			p.text("  挂载：", strings.Join(disk.MountPoints, ", "))
			p.text("  项目：", strings.Join(disk.ProjectIDs, ", "))
			if disk.Error != "" {
				p.text("  错误：", disk.Error)
			}
		}
	}
	p.line("")
	p.text("", fmt.Sprintf("合计：%d 项目，%d 个受管磁盘/容器", r.Summary.ProjectCount, r.Summary.DiskCount))
	p.metrics([]string{"总容量", "空闲", "可用空间"}, []string{humanBytes(r.Summary.TotalBytes), humanBytes(r.Summary.FreeBytes), humanBytes(r.Summary.AvailableAfterReservationsBytes)})
	p.line("")
	p.text("说明：", "可用空间已扣除预留及安全余量，不等于可上传文件大小。")
	p.text("      ", "仅统计项目所在磁盘，共享空间只计一次；未知项会使合计显示未知。")
	p.text("      ", "只读映射不扫描文件占用。")
	return p.err
}
