package kero

import (
	"testing"
	"time"
)

func TestSelectTimeUnitForTimeframe(t *testing.T) {
	loc, _ := time.LoadLocation("Europe/Zurich")

	cases := []struct {
		start, end time.Time
		wants      time.Duration
	}{
		// today - "t"/"24h"
		{
			start: time.Date(2023, time.August, 9, 0, 0, 0, 0, loc),
			end:   time.Date(2023, time.August, 9, 10, 11, 12, 13, loc),
			wants: AggregateByHour,
		},
		// last 7 days - "7d"
		{
			start: time.Date(2023, time.August, 2, 0, 0, 0, 0, loc),
			end:   time.Date(2023, time.August, 9, 10, 11, 12, 13, loc),
			wants: AggregateByDay,
		},
		// last 9 days
		{
			start: time.Date(2023, time.August, 1, 2, 3, 4, 5, loc),
			end:   time.Date(2023, time.August, 9, 10, 11, 12, 13, loc),
			wants: AggregateByDay,
		},
		// last 30 days - "30d"
		{
			start: time.Date(2023, time.July, 10, 10, 11, 12, 13, loc),
			end:   time.Date(2023, time.August, 9, 10, 11, 12, 13, loc),
			wants: AggregateByDay,
		},
		// last 60 days
		{
			start: time.Date(2023, time.June, 9, 10, 11, 12, 13, loc),
			end:   time.Date(2023, time.August, 9, 10, 11, 12, 13, loc),
			wants: AggregateByWeek,
		},
		// last 12 months - "12m"
		{
			start: time.Date(2022, time.June, 9, 10, 11, 12, 13, loc),
			end:   time.Date(2023, time.August, 9, 10, 11, 12, 13, loc),
			wants: AggregateByMonth,
		},
	}

	for i, testCase := range cases {
		got := selectTimeUnitForTimeframe(testCase.start.Unix(), testCase.end.Unix())
		if got != testCase.wants {
			t.Fatal("Case", i, ": Selected", got, "aggregation instead of", testCase.wants, ", for", testCase.start, testCase.end)
		}
	}
}
