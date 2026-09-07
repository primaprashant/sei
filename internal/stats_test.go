package app

import (
	"encoding/json/v2"
	"math"
	"reflect"
	"strings"
	"testing"
	"time"
)

const sampleStats = `{"version":1,"tracking_started":"2026-09-07","days":{"2026-09-07":{"removals":2,"copies_by_skill":{"code-review":3,"debugging":2,"writing":1}}}}`

func TestStatsSummary(t *testing.T) {
	history := statsHistory{Version: 1, TrackingStarted: "2026-08-08", Days: map[string]statsDay{
		"2026-08-08": {0, map[string]int64{"alpha": 10}},
		"2026-08-09": {0, map[string]int64{"beta": 3}},
		"2026-08-31": {0, map[string]int64{"alpha": 2}},
		"2026-09-01": {0, map[string]int64{"gamma": 4}},
		"2026-09-07": {2, map[string]int64{"beta": 1}},
		"2026-09-08": {0, map[string]int64{"future": 5}},
	}}
	now := time.Date(2026, 9, 7, 12, 0, 0, 0, time.UTC)
	got := history.summarize(now)
	want := statsSummary{copies: 25, removals: 2, activeDays: 6, monthActions: 7,
		allTime: []skillCount{{"alpha", 12}, {"future", 5}, {"beta", 4}, {"gamma", 4}},
		recent:  []skillCount{{"beta", 4}, {"gamma", 4}, {"alpha", 2}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("summary = %+v, want %+v", got, want)
	}
}

func TestStatsCalendarBoundaries(t *testing.T) {
	for _, zone := range []string{"America/New_York", "America/Santiago", "Asia/Tokyo"} {
		location, err := time.LoadLocation(zone)
		if err != nil {
			t.Fatal(err)
		}
		for _, date := range []string{"2026-01-01", "2026-03-09", "2026-10-05", "2026-11-02", "2028-03-01"} {
			t.Run(zone+"/"+date, func(t *testing.T) {
				now, err := time.ParseInLocation("2006-01-02 15:04", date+" 00:30", location)
				if err != nil {
					t.Fatal(err)
				}
				var h statsHistory
				calendar, err := time.Parse(time.DateOnly, date)
				if err != nil {
					t.Fatal(err)
				}
				for _, offset := range []int{-30, -29, 0, 1} {
					day := calendar.AddDate(0, 0, offset)
					noon := time.Date(day.Year(), day.Month(), day.Day(), 12, 0, 0, 0, location)
					if err := h.record("skill", true, noon); err != nil {
						t.Fatal(err)
					}
				}
				summary := h.summarize(now)
				if summary.copies != 4 || summary.recent[0].count != 2 {
					t.Fatalf("calendar window: %+v", summary)
				}
				if strings.HasSuffix(date, "-01") && summary.monthActions != 1 {
					t.Fatalf("month included previous month or tomorrow: %+v", summary)
				}
			})
		}
	}
}

func TestStatsValidation(t *testing.T) {
	if _, err := parseStats([]byte(sampleStats)); err != nil {
		t.Fatal(err)
	}
	for _, data := range []string{
		"", "null", "{}", sampleStats + "{}",
		strings.Replace(sampleStats, `"version":1`, `"version":2`, 1),
		strings.Replace(sampleStats, `"version":1`, `"Version":1`, 1),
		strings.Replace(sampleStats, `"version":1`, `"version":1,"version":1`, 1),
		strings.Replace(sampleStats, `"version":1`, `"version":1,"extra":true`, 1),
		strings.Replace(sampleStats, `"removals":2`, `"removals":-1`, 1),
		strings.Replace(sampleStats, `"removals":2`, `"removals":null`, 1),
		strings.Replace(sampleStats, `"removals":2`, `"removals":9223372036854775807`, 1),
		strings.Replace(sampleStats, `"code-review":3`, `"code-review":0`, 1),
		strings.Replace(sampleStats, `"code-review":3`, `"code-review":1.5`, 1),
		strings.Replace(sampleStats, `"code-review":3`, `"code-review":null`, 1),
		strings.Replace(sampleStats, `"code-review":3`, `"code-review":3,"code-review":4`, 1),
		strings.Replace(sampleStats, `"code-review"`, `"../escape"`, 1),
		strings.Replace(sampleStats, `"code-review"`, `"bad\\escape"`, 1),
		strings.Replace(sampleStats, "2026-09-07", "2026-09-08", 1),
		strings.ReplaceAll(sampleStats, "2026-09-07", "2026-02-30"),
		`{"version":1,"tracking_started":"2026-09-07","days":{}}`,
		`{"version":1,"tracking_started":"2026-09-07","days":{"2026-09-07":null}}`,
	} {
		if _, err := parseStats([]byte(data)); err == nil {
			t.Fatalf("accepted invalid history: %s", data)
		}
	}
}

func TestStatsNamesAndCounts(t *testing.T) {
	now := time.Date(2026, 9, 7, 23, 59, 0, 0, time.FixedZone("test", 9*60*60))
	names := []string{"plain", "line\nname", `line\nname`, "bad\xff", "bad\xfe", "界", "quote\"name", "\x1b[2J"}
	var h statsHistory
	for _, name := range names {
		if err := h.record(name, true, now); err != nil {
			t.Fatal(err)
		}
	}
	if err := h.record("plain", false, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(h)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := parseStats(data)
	if err != nil || !reflect.DeepEqual(loaded, h) {
		t.Fatalf("lost names: %+v, %v", loaded, err)
	}
	if len(loaded.Days["2026-09-07"].CopiesBySkill) != len(names) || loaded.Days["2026-09-08"].Removals != 1 || len(loaded.summarize(now).allTime) != 5 {
		t.Fatalf("incorrect grouping or ranking: %+v", loaded)
	}
	h = statsHistory{Version: 1, TrackingStarted: "2026-09-07", Days: map[string]statsDay{"2026-09-07": {math.MaxInt64, map[string]int64{}}}}
	if err := h.record("plain", true, now); err == nil {
		t.Fatal("counter overflow accepted")
	}
}
