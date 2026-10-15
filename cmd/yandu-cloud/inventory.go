package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"time"
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
