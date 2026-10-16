package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"
	"unicode"
	"unicode/utf8"

	"yandu/internal/inventory"
)

func presentationFixture() inventory.Result {
	projects := []inventory.Project{
		{ID: "photos", Name: "家庭相册", Mode: "application_storage", State: "active", StorageState: "enabled", RootPath: "/Volumes/本地素材盘/家庭 相册", DiskID: "apfs-container-demo", QuotaBytes: inventory.Bytes(100 << 30), UsedBytes: inventory.Bytes(12345 << 20), ReservedBytes: inventory.Bytes(256 << 20), RemainingQuotaBytes: inventory.Bytes(898 << 26)},
		{ID: "a-project-with-a-very-long-but-valid-identifier", Name: "视频素材 👨‍👩‍👧‍👦 café", Mode: "application_storage", State: "disabled", StorageState: "quarantined", RootPath: "/Volumes/" + strings.Repeat("很长的中文目录/", 10) + "包含 空格的原件", DiskID: "apfs-container-demo", QuotaBytes: inventory.Bytes(500 << 30), Error: "VOLUME_CHANGED: volume identity differs from the registered storage root"},
		{ID: "blog", Name: "个人博客", Mode: "readonly_mapping", State: "draft", RootPath: "/Volumes/网站图片/blog"},
	}
	report := inventory.Report{SchemaVersion: 1, DeviceID: "owner", Platform: "darwin/arm64", SampledAt: time.Date(2026, 10, 15, 8, 30, 0, 0, time.UTC), Projects: projects, Disks: []inventory.Disk{
		{ID: "apfs-container-demo", Filesystem: "APFS", MountPoints: []string{"/Volumes/本地素材盘", "/Volumes/共享存储盘"}, ProjectIDs: []string{projects[0].ID, projects[1].ID}, TotalBytes: inventory.Bytes(2 << 40), FreeBytes: inventory.Bytes(1 << 40), ReservedBytes: 256 << 20, AvailableAfterReservationsBytes: inventory.Bytes((1 << 40) - (320 << 20))},
	}}
	report.Summarize()
	return inventory.Result{Report: report, Online: true}
}

func TestInventoryTableChineseAndNumericAlignment(t *testing.T) {
	var out bytes.Buffer
	p := &inventoryPrinter{out: &out, width: 80}
	if !p.table([]inventoryColumn{{"名称", 4, 8, false}, {"ID", 2, 8, false}, {"已用", 4, 8, true}}, []inventoryRow{{values: []string{"图库", "photos", "1 MiB"}}, {values: []string{"abc", "x", "9 GiB"}}}) {
		t.Fatal("small table should fit")
	}
	want := `┌──────┬────────┬───────┐
│ 名称 │ ID     │  已用 │
├──────┼────────┼───────┤
│ 图库 │ photos │ 1 MiB │
├──────┼────────┼───────┤
│ abc  │ x      │ 9 GiB │
└──────┴────────┴───────┘
`
	if out.String() != want {
		t.Fatalf("columns are not aligned by terminal cells:\n%s\nwant:\n%s", out.String(), want)
	}
}

func TestInventoryDetailsStayWithTheirRecordAndUseAvailableWidth(t *testing.T) {
	var out bytes.Buffer
	p := &inventoryPrinter{out: &out, width: 100}
	path := "/Users/example/Documents/projects/browser-demo/storage"
	if !p.table([]inventoryColumn{{"项目", 4, 12, false}, {"已用", 4, 12, true}}, []inventoryRow{
		{[]string{"中文名称\nproject-one", "1.00 GiB"}, []inventoryDetail{{"目录：", path, false}}},
		{[]string{"第二个项目", "1023 B"}, []inventoryDetail{{"目录：", "/second", false}}},
	}) {
		t.Fatal("table should fit")
	}
	text := out.String()
	if !strings.Contains(text, path) {
		t.Fatal("path wrapped despite available terminal space")
	}
	first, second := strings.Index(text, path), strings.Index(text, "第二个项目")
	if first < 0 || first >= second || !strings.Contains(text[:first], "┴") || !strings.Contains(text[first:second], "┬") {
		t.Fatal("details are not enclosed with their own record")
	}
	for _, line := range strings.Split(strings.TrimSpace(text), "\n") {
		if cells.StringWidth(line) != cells.StringWidth(strings.Split(text, "\n")[0]) {
			t.Fatalf("unequal row width: %q", line)
		}
	}
}

func TestInventoryPresentationFitsTerminalAndKeepsFullValues(t *testing.T) {
	r := presentationFixture()
	compact := func(s string) string {
		return strings.Map(func(r rune) rune {
			if unicode.IsSpace(r) {
				return -1
			}
			return r
		}, s)
	}
	// Reassemble each logical column across wrapped physical lines. Border
	// characters and other columns must not interrupt a name or identifier.
	contents := func(text string) string {
		var columns [8]string
		for _, line := range strings.Split(text, "\n") {
			if !strings.HasPrefix(line, "│") {
				continue
			}
			parts := strings.Split(line, "│")
			for i, part := range parts[1 : len(parts)-1] {
				columns[i] += compact(part)
			}
		}
		return compact(text) + strings.Join(columns[:], "\n")
	}
	for _, width := range []int{24, 40, 80, 100, 120, 180} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			var out bytes.Buffer
			if err := printInventoryWidth(&out, r, false, width); err != nil {
				t.Fatal(err)
			}
			text := out.String()
			fullValues := contents(text)
			if !utf8.ValidString(text) || strings.ContainsAny(text, "\t\x1b") {
				t.Fatal("invalid terminal text")
			}
			for _, line := range strings.Split(text, "\n") {
				if cells.StringWidth(line) > width {
					t.Fatalf("line exceeds %d terminal columns (%d): %q", width, cells.StringWidth(line), line)
				}
			}
			for _, project := range r.Projects {
				for _, value := range []string{project.ID, project.Name, project.RootPath, project.Error} {
					if !strings.Contains(fullValues, compact(value)) {
						t.Fatalf("complete value lost at width %d: %q", width, value)
					}
				}
			}
			if width == 40 && !strings.Contains(text, "│ ID：photos") {
				t.Fatal("narrow terminal should use bordered project cards")
			}
			if width >= 80 && !strings.Contains(text, "│ 项目 / ID") {
				t.Fatal("ordinary terminals should keep a bordered project table")
			}
			borderWidth := 0
			for _, line := range strings.Split(text, "\n") {
				if strings.HasPrefix(line, "┌") {
					borderWidth = cells.StringWidth(line)
				}
				if strings.ContainsAny(line, "│├└") && cells.StringWidth(line) != borderWidth {
					t.Fatalf("broken border at width %d: %q", width, line)
				}
			}
		})
	}
}

func TestInventoryPresentationEmptyOfflineAndDiskViews(t *testing.T) {
	for _, tc := range []struct {
		name         string
		result       inventory.Result
		disksOnly    bool
		want, absent string
	}{
		{"empty", inventory.Result{Online: true}, false, "暂无项目", "项目 / ID"},
		{"offline", inventory.Result{Error: "DEVICE_OFFLINE"}, false, "没有可用的设备快照", "磁盘 / 共享容器"},
		{"disks", presentationFixture(), true, "APFS", "家庭相册"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			if err := printInventoryWidth(&out, tc.result, tc.disksOnly, 80); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(out.String(), tc.want) || strings.Contains(out.String(), tc.absent) {
				t.Fatal(out.String())
			}
		})
	}
	r := presentationFixture()
	r.Online = false
	r.Stale = true
	var out bytes.Buffer
	if err := printInventoryWidth(&out, r, false, 80); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "历史快照") || !strings.Contains(out.String(), "不可作为当前写入依据") {
		t.Fatal("stale capacities lost their warning")
	}
}

func TestInventoryWidthAndOutputErrors(t *testing.T) {
	for _, tc := range []struct {
		env  string
		want int
	}{{"80", 80}, {"0", 120}, {"garbage", 120}, {"10", 24}, {"300", 180}} {
		t.Setenv("COLUMNS", tc.env)
		if width := inventoryWidth(&bytes.Buffer{}); width != tc.want {
			t.Fatalf("COLUMNS=%s: width %d", tc.env, width)
		}
	}
	if err := printInventoryWidth(closedInventoryWriter{}, presentationFixture(), false, 80); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("write error lost: %v", err)
	}
	if text := cleanCell("中文\t名称\n\x1b[31m\u202e👨‍👩‍👧‍👦"); strings.ContainsAny(text, "\t\n\x1b\u202e") || !strings.Contains(text, "👨‍👩‍👧‍👦") {
		t.Fatalf("control characters or grapheme handling: %q", text)
	}
}

type closedInventoryWriter struct{}

func (closedInventoryWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }
