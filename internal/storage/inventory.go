package storage

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"net/http"
	"runtime"
	"sort"
	"strings"
	"time"
	"yandu/internal/inventory"
	"yandu/internal/model"
	"yandu/internal/platform"
	"yandu/internal/secrets"
)

func (s *Service) serveInventory(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if r.Method != http.MethodGet || r.URL.RawQuery != "" {
		reject(w, fail("INVALID_REQUEST", "inventory only supports GET without query parameters", 405))
		return
	}
	if s.InventorySource == nil {
		reject(w, fail("MONITOR_NOT_CONFIGURED", "import a monitoring-enabled storage profile", 503))
		return
	}
	snapshot, e := s.InventorySource()
	if e != nil {
		reject(w, e)
		return
	}
	if snapshot.Profile == nil || snapshot.Profile.Storage == nil || !secrets.ValidMonitorToken(snapshot.Profile.Storage.MonitorToken) {
		reject(w, fail("MONITOR_NOT_CONFIGURED", "import a monitoring-enabled storage profile", 503))
		return
	}
	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if !secrets.ValidMonitorToken(token) || subtle.ConstantTimeCompare([]byte(token), []byte(snapshot.Profile.Storage.MonitorToken)) != 1 {
		reject(w, fail("UNAUTHORIZED", "device monitoring credential required", 401))
		return
	}
	report, e := s.collectInventory(r.Context(), snapshot, platform.CheckRoot)
	if e != nil {
		reject(w, e)
		return
	}
	// Profile rotation during sampling revokes the previous credential immediately.
	latest, e := s.InventorySource()
	if e != nil || latest.Profile == nil || latest.Profile.Storage == nil || subtle.ConstantTimeCompare([]byte(token), []byte(latest.Profile.Storage.MonitorToken)) != 1 {
		reject(w, fail("UNAUTHORIZED", "monitoring credential changed", 401))
		return
	}
	respond(w, 200, report)
}

type inventoryRecord struct {
	project        Project
	used, reserved int64
}

func (s *Service) collectInventory(ctx context.Context, snapshot model.Snapshot, inspect func(string) (platform.Volume, error)) (inventory.Report, error) {
	report := inventory.Report{SchemaVersion: 1, DeviceID: snapshot.Profile.DeviceID, Platform: runtime.GOOS + "/" + runtime.GOARCH, Projects: []inventory.Project{}, Disks: []inventory.Disk{}}
	// Read usage and reservation counters in one SQLite transaction. Do not hold
	// the database connection while running platform disk inspection commands.
	tx, e := s.DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if e != nil {
		return report, e
	}
	defer tx.Rollback()
	rows, e := tx.QueryContext(ctx, `SELECT p.id,p.root,p.backend_key,p.quota,p.enabled,p.volume,p.pool,p.quarantined,
	 COALESCE((SELECT SUM(size) FROM versions WHERE project=p.id),0)+COALESCE((SELECT SUM(size) FROM derivatives WHERE project=p.id),0),
	 COALESCE((SELECT SUM(bytes) FROM reservations WHERE project=p.id),0) FROM projects p`)
	if e != nil {
		return report, e
	}
	records := map[string]inventoryRecord{}
	for rows.Next() {
		var v inventoryRecord
		p := &v.project
		if e = rows.Scan(&p.ID, &p.Root, &p.Key, &p.Quota, &p.Enabled, &p.Volume, &p.Pool, &p.Quarantined, &v.used, &v.reserved); e != nil {
			rows.Close()
			return report, e
		}
		records[p.ID] = v
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return report, e
	}
	rows, e = tx.QueryContext(ctx, "SELECT pool,SUM(bytes) FROM reservations GROUP BY pool")
	if e != nil {
		return report, e
	}
	poolReservations := map[string]int64{}
	for rows.Next() {
		var pool string
		var n int64
		if e = rows.Scan(&pool, &n); e != nil {
			rows.Close()
			return report, e
		}
		poolReservations[pool] = n
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return report, e
	}
	if e = tx.Commit(); e != nil {
		return report, e
	}
	disks := map[string]*inventory.Disk{}
	projects := append([]model.Project{}, snapshot.Projects...)
	sort.Slice(projects, func(i, j int) bool { return projects[i].ID < projects[j].ID })
	for _, configured := range projects {
		if e = ctx.Err(); e != nil {
			return report, e
		}
		p := inventory.Project{ID: configured.ID, Name: configured.DisplayName, Mode: configured.Mode, State: configured.Status, RootPath: configured.RootPath}
		if p.Mode == "" {
			p.Mode = "readonly_mapping"
		}
		stored, initialized := records[p.ID]
		if configured.IsStorage() {
			p.QuotaBytes = inventory.Bytes(configured.QuotaBytes)
			p.StorageState = "uninitialized"
			if initialized {
				p.QuotaBytes = inventory.Bytes(stored.project.Quota)
				p.UsedBytes = inventory.Bytes(stored.used)
				p.ReservedBytes = inventory.Bytes(stored.reserved)
				p.RemainingQuotaBytes = inventory.Bytes(inventory.Nonnegative(stored.project.Quota - stored.used - stored.reserved))
				p.VolumeID = stored.project.Volume
				p.DiskID = stored.project.Pool
				p.RootPath = stored.project.Root
				p.StorageState = "disabled"
				if stored.project.Enabled {
					p.StorageState = "enabled"
				}
				if stored.project.Quarantined {
					p.StorageState = "quarantined"
				}
			}
		}
		volume, err := inspect(p.RootPath)
		if err != nil {
			p.Error = "VOLUME_UNAVAILABLE"
		} else if initialized && configured.IsStorage() && (volume.ID != stored.project.Volume || volume.Pool != stored.project.Pool) {
			p.Error = "VOLUME_CHANGED"
		} else {
			p.VolumeID = volume.ID
			p.DiskID = volume.Pool
		}
		if p.DiskID != "" {
			d := disks[p.DiskID]
			if d == nil {
				d = &inventory.Disk{ID: p.DiskID, MountPoints: []string{}, VolumeIDs: []string{}, ProjectIDs: []string{}, ReservedBytes: poolReservations[p.DiskID]}
				disks[p.DiskID] = d
			}
			d.ProjectIDs = append(d.ProjectIDs, p.ID)
			if p.Error != "" {
				d.Error = p.Error
				d.VolumeIDs = uniqueString(d.VolumeIDs, p.VolumeID)
			} else {
				d.Filesystem = volume.Filesystem
				d.MountPoints = uniqueString(d.MountPoints, volume.Mount)
				d.VolumeIDs = uniqueString(d.VolumeIDs, volume.ID)
				d.TotalBytes = inventory.Bytes(volume.Total)
				if d.FreeBytes == nil || volume.Free < *d.FreeBytes {
					d.FreeBytes = inventory.Bytes(volume.Free)
				}
			}
		}
		report.Projects = append(report.Projects, p)
	}
	for _, d := range disks {
		if d.Error != "" {
			d.TotalBytes = nil
			d.FreeBytes = nil
		} else if d.FreeBytes != nil {
			d.AvailableAfterReservationsBytes = inventory.Bytes(inventory.Nonnegative(*d.FreeBytes - d.ReservedBytes - inventory.SafetyBytes))
		}
		sort.Strings(d.MountPoints)
		sort.Strings(d.VolumeIDs)
		report.Disks = append(report.Disks, *d)
	}
	sort.Slice(report.Disks, func(i, j int) bool { return report.Disks[i].ID < report.Disks[j].ID })
	report.SampledAt = time.Now().UTC()
	report.Summarize()
	return report, nil
}

func uniqueString(values []string, v string) []string {
	if v == "" {
		return values
	}
	for _, old := range values {
		if old == v {
			return values
		}
	}
	return append(values, v)
}
