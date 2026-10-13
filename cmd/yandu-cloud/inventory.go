package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
	"time"
	"unicode"
	"yandu/internal/cloud"
	"yandu/internal/inventory"
)

func inventoryCommand(command string, args []string, out, errOut io.Writer) error {
	fs := flag.NewFlagSet(command, flag.ContinueOnError)
	fs.SetOutput(errOut)
	config := fs.String("config", "/etc/yandu/monitor.json", "只读监控配置")
	jsonOutput := fs.Bool("json", false, "输出 JSON")
	timeout := fs.Duration("timeout", 30*time.Second, "完整设备采样超时")
	if e := fs.Parse(args); e != nil {
		return e
	}
	if fs.NArg() != 0 || *timeout <= 0 || *timeout > 5*time.Minute {
		return fmt.Errorf("用法：yandu %s [--json] [--config PATH] [--timeout 30s]", command)
	}
	c, e := cloud.LoadMonitorConfig(*config)
	if e != nil {
		return e
	}
	report, fetchError := cloud.FetchInventory(cloud.MonitorClient(*timeout), c)
	result := inventory.Result{Report: report, Online: fetchError == nil}
	if fetchError == nil {
		if e = cloud.SaveInventory(*config, c, report); e != nil {
			fmt.Fprintln(errOut, "提示：实时查询成功，但无法保存离线快照；请检查配置目录写权限。")
		}
	} else {
		result.Report = inventory.Report{SchemaVersion: 1, DeviceID: c.DeviceID, Projects: []inventory.Project{}, Disks: []inventory.Disk{}}
		result.Error = fetchError.Error()
		var me *cloud.MonitorError
		if errors.As(fetchError, &me) && me.CanUseCache {
			if cached, e := cloud.CachedInventory(*config, c); e == nil {
				result.Report = cached
				result.Stale = true
			}
		}
	}
	if !result.SampledAt.IsZero() {
		result.SnapshotAgeSeconds = inventory.Nonnegative(int64(time.Since(result.SampledAt).Seconds()))
	}
	if *jsonOutput {
		e = json.NewEncoder(out).Encode(result)
	} else {
		e = printInventory(out, result, command == "disks")
	}
	if e != nil {
		return e
	}
	return fetchError
}

func cleanCell(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, s)
}
func humanBytes(n *int64) string {
	if n == nil {
		return "未知"
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
func printInventory(out io.Writer, r inventory.Result, disksOnly bool) error {
	state := "在线"
	if !r.Online {
		state = "不可达"
	}
	if r.Stale {
		state = "不可达（下列为历史快照，容量不可作为当前写入依据）"
	}
	fmt.Fprintf(out, "设备：%s  平台：%s  状态：%s\n", cleanCell(r.DeviceID), cleanCell(r.Platform), state)
	if !r.SampledAt.IsZero() {
		fmt.Fprintf(out, "采样：%s  快照年龄：%ds\n", r.SampledAt.Format(time.RFC3339), r.SnapshotAgeSeconds)
	}
	if r.Error != "" {
		fmt.Fprintln(out, "查询：", cleanCell(r.Error))
	}
	if !r.Online && !r.Stale {
		fmt.Fprintln(out, "没有可用的设备快照。")
		return nil
	}
	w := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
	diskLabels := map[string]string{}
	for i, d := range r.Disks {
		diskLabels[d.ID] = fmt.Sprintf("D%d", i+1)
	}
	if !disksOnly {
		fmt.Fprintln(w, "\n项目 ID\t名称\t模式 / 状态\t磁盘\t配额\t已用\t预留\t剩余额度")
		for _, p := range r.Projects {
			disk := diskLabels[p.DiskID]
			if disk == "" {
				disk = "未知"
			}
			mode := p.Mode
			if mode == "application_storage" {
				mode = "存储"
			}
			if mode == "readonly_mapping" {
				mode = "只读映射"
			}
			state := p.State
			if p.StorageState != "" {
				state += " / " + p.StorageState
			}
			if p.Error != "" {
				state += " / " + p.Error
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n", cleanCell(p.ID), cleanCell(p.Name), cleanCell(mode+" / "+state), disk, humanBytes(p.QuotaBytes), humanBytes(p.UsedBytes), humanBytes(p.ReservedBytes), humanBytes(p.RemainingQuotaBytes))
			fmt.Fprintf(w, "  目录：%s\n", cleanCell(p.RootPath))
		}
	}
	fmt.Fprintln(w, "\n磁盘 / 共享容器\t挂载点\t文件系统\t总容量\t空闲\t预留\t扣除预留及安全余量后\t状态")
	for _, d := range r.Disks {
		state := "可查询"
		if d.Error != "" {
			state = d.Error
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n", diskLabels[d.ID], cleanCell(strings.Join(d.MountPoints, ", ")), cleanCell(d.Filesystem), humanBytes(d.TotalBytes), humanBytes(d.FreeBytes), humanBytes(inventory.Bytes(d.ReservedBytes)), humanBytes(d.AvailableAfterReservationsBytes), cleanCell(state))
		fmt.Fprintf(w, "  标识：%s  项目：%s\n", cleanCell(d.ID), cleanCell(strings.Join(d.ProjectIDs, ", ")))
	}
	if e := w.Flush(); e != nil {
		return e
	}
	fmt.Fprintf(out, "\n合计：%d 项目，%d 个受管磁盘/容器；磁盘总容量 %s，空闲 %s，扣除预留及安全余量后 %s\n", r.Summary.ProjectCount, r.Summary.DiskCount, humanBytes(r.Summary.TotalBytes), humanBytes(r.Summary.FreeBytes), humanBytes(r.Summary.AvailableAfterReservationsBytes))
	fmt.Fprintln(out, "统计仅覆盖项目所在磁盘；共享空间只计算一次。未知项目/磁盘会使合计显示未知。只读映射不扫描文件占用；可用空间不等于可上传文件大小。")
	return nil
}
