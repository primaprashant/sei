package app

import (
	"encoding/json/v2"
	"fmt"
	"math"
	"sort"
	"strconv"
	"time"
)

type statsHistory struct {
	Version         int                 `json:"version"`
	TrackingStarted string              `json:"tracking_started"`
	Days            map[string]statsDay `json:"days"`
}

type statsDay struct {
	Removals      int64            `json:"removals"`
	CopiesBySkill map[string]int64 `json:"copies_by_skill"`
}

type skillCount struct {
	name  string // Raw filename, decoded from the persisted key.
	count int64
}

type statsSummary struct {
	copies, removals, monthActions int64
	activeDays                     int
	allTime, recent                []skillCount
}

func parseStats(data []byte) (statsHistory, error) {
	var history statsHistory
	counts := json.UnmarshalFunc(func(data []byte, count *int64) error {
		if string(data) == "null" {
			return fmt.Errorf("stats counts must be integers")
		}
		return json.Unmarshal(data, count)
	})
	if err := json.Unmarshal(data, &history, json.RejectUnknownMembers(true), json.WithUnmarshalers(counts)); err != nil {
		return statsHistory{}, err
	}
	if history.Version != 1 {
		return statsHistory{}, fmt.Errorf("unsupported stats version %d", history.Version)
	}
	if _, err := time.Parse(time.DateOnly, history.TrackingStarted); err != nil {
		return statsHistory{}, fmt.Errorf("invalid tracking start date")
	}
	if len(history.Days) == 0 {
		return statsHistory{}, fmt.Errorf("stats history has no active days")
	}
	var total int64
	add := func(n int64) bool {
		if n < 0 || n > math.MaxInt64-total {
			return false
		}
		total += n
		return true
	}
	for date, day := range history.Days {
		if _, err := time.Parse(time.DateOnly, date); err != nil || date < history.TrackingStarted {
			return statsHistory{}, fmt.Errorf("invalid activity date %q", date)
		}
		if day.CopiesBySkill == nil || (day.Removals == 0 && len(day.CopiesBySkill) == 0) || !add(day.Removals) {
			return statsHistory{}, fmt.Errorf("invalid daily counts for %s", date)
		}
		for key, count := range day.CopiesBySkill {
			name, err := strconv.Unquote(`"` + key + `"`)
			if err != nil || validateSkillName(name) != nil || statsSkillKey(name) != key || count <= 0 || !add(count) {
				return statsHistory{}, fmt.Errorf("invalid skill count for %s", date)
			}
		}
	}
	return history, nil
}

// Quoting preserves invalid UTF-8 and distinguishes literal backslashes from
// escape sequences. Ordinary names remain readable JSON object keys.
func statsSkillKey(name string) string {
	quoted := strconv.Quote(name)
	return quoted[1 : len(quoted)-1]
}

func (h *statsHistory) record(name string, copy bool, now time.Time) error {
	if err := validateSkillName(name); err != nil {
		return err
	}
	if summary := h.summarize(now); summary.copies+summary.removals == math.MaxInt64 {
		return fmt.Errorf("stats counter limit reached")
	}
	date := now.Format(time.DateOnly)
	if h.Days == nil {
		h.Version, h.Days = 1, make(map[string]statsDay)
	}
	if h.TrackingStarted == "" || date < h.TrackingStarted {
		h.TrackingStarted = date
	}
	day := h.Days[date]
	if day.CopiesBySkill == nil {
		day.CopiesBySkill = make(map[string]int64)
	}
	if copy {
		day.CopiesBySkill[statsSkillKey(name)]++
	} else {
		day.Removals++
	}
	h.Days[date] = day
	return nil
}

func (h statsHistory) summarize(now time.Time) statsSummary {
	var result statsSummary
	all, recent := make(map[string]int64), make(map[string]int64)
	today := now.Format(time.DateOnly)
	// Shift the local date in UTC: a local midnight can disappear during DST.
	calendar := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	start := calendar.AddDate(0, 0, -29).Format(time.DateOnly)
	month := now.Format("2006-01") + "-01"
	for date, day := range h.Days {
		var copies int64
		for key, count := range day.CopiesBySkill {
			copies += count
			all[key] += count
			if date >= start && date <= today {
				recent[key] += count
			}
		}
		result.copies += copies
		result.removals += day.Removals
		if copies+day.Removals > 0 {
			result.activeDays++
		}
		if date >= month && date <= today {
			result.monthActions += copies + day.Removals
		}
	}
	result.allTime, result.recent = topSkills(all), topSkills(recent)
	return result
}

func topSkills(counts map[string]int64) []skillCount {
	items := make([]skillCount, 0, len(counts))
	for key, count := range counts {
		name, _ := strconv.Unquote(`"` + key + `"`) // History validation checked the key.
		items = append(items, skillCount{name, count})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].count == items[j].count {
			return items[i].name < items[j].name
		}
		return items[i].count > items[j].count
	})
	return items[:min(5, len(items))]
}
