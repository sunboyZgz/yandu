// Package inventory defines the read-only device inventory shared by the local
// agent and cloud CLI. It contains no file credentials or tunnel secrets.
package inventory

import "time"

const SafetyBytes int64 = 64 << 20

type Project struct {
	ID                  string `json:"id"`
	Name                string `json:"name"`
	Mode                string `json:"mode"`
	State               string `json:"state"`
	StorageState        string `json:"storageState,omitempty"`
	RootPath            string `json:"rootPath"`
	VolumeID            string `json:"volumeId,omitempty"`
	DiskID              string `json:"diskId,omitempty"`
	QuotaBytes          *int64 `json:"quotaBytes"`
	UsedBytes           *int64 `json:"usedBytes"`
	ReservedBytes       *int64 `json:"reservedBytes"`
	RemainingQuotaBytes *int64 `json:"remainingQuotaBytes"`
	Error               string `json:"error,omitempty"`
}

// A disk is an allocation pool: a Windows NTFS volume or a shared APFS
// container. Multiple volumes in the same APFS container count only once.
type Disk struct {
	ID                              string   `json:"id"`
	Filesystem                      string   `json:"filesystem"`
	MountPoints                     []string `json:"mountPoints"`
	VolumeIDs                       []string `json:"volumeIds"`
	ProjectIDs                      []string `json:"projectIds"`
	TotalBytes                      *int64   `json:"totalBytes"`
	FreeBytes                       *int64   `json:"freeBytes"`
	ReservedBytes                   int64    `json:"reservedBytes"`
	AvailableAfterReservationsBytes *int64   `json:"availableAfterReservationsBytes"`
	Error                           string   `json:"error,omitempty"`
}

type Summary struct {
	ProjectCount                    int    `json:"projectCount"`
	DiskCount                       int    `json:"diskCount"`
	UnavailableDiskCount            int    `json:"unavailableDiskCount"`
	UnassignedProjectCount          int    `json:"unassignedProjectCount"`
	StorageUsedBytes                int64  `json:"storageUsedBytes"`
	ReservedBytes                   int64  `json:"reservedBytes"`
	TotalBytes                      *int64 `json:"totalBytes"`
	FreeBytes                       *int64 `json:"freeBytes"`
	AvailableAfterReservationsBytes *int64 `json:"availableAfterReservationsBytes"`
}

type Report struct {
	SchemaVersion int       `json:"schemaVersion"`
	DeviceID      string    `json:"deviceId"`
	Platform      string    `json:"platform"`
	SampledAt     time.Time `json:"sampledAt,omitzero"`
	Projects      []Project `json:"projects"`
	Disks         []Disk    `json:"disks"`
	Summary       Summary   `json:"summary"`
}

type Result struct {
	Report
	Online             bool   `json:"online"`
	Stale              bool   `json:"stale"`
	SnapshotAgeSeconds int64  `json:"snapshotAgeSeconds"`
	Error              string `json:"error,omitempty"`
}

func Bytes(n int64) *int64 { return &n }
func Nonnegative(n int64) int64 {
	if n < 0 {
		return 0
	}
	return n
}

func (r *Report) Summarize() {
	s := Summary{ProjectCount: len(r.Projects), DiskCount: len(r.Disks)}
	for _, p := range r.Projects {
		if p.DiskID == "" {
			s.UnassignedProjectCount++
		}
		if p.UsedBytes != nil {
			s.StorageUsedBytes += *p.UsedBytes
		}
	}
	var total, free, available int64
	for _, d := range r.Disks {
		s.ReservedBytes += d.ReservedBytes
		if d.Error != "" || d.TotalBytes == nil || d.FreeBytes == nil || d.AvailableAfterReservationsBytes == nil {
			s.UnavailableDiskCount++
			continue
		}
		total += *d.TotalBytes
		free += *d.FreeBytes
		available += *d.AvailableAfterReservationsBytes
	}
	// Incomplete totals are unknown, not falsely reported as a full-device total.
	if len(r.Disks) > 0 && s.UnavailableDiskCount == 0 && s.UnassignedProjectCount == 0 {
		s.TotalBytes = Bytes(total)
		s.FreeBytes = Bytes(free)
		s.AvailableAfterReservationsBytes = Bytes(available)
	}
	r.Summary = s
}
