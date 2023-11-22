package kero

import (
	"context"
	"sort"
	"strings"
	"time"

	plabels "github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/tsdb/chunkenc"
)

type Metric struct {
	Ts     int64        `json:"timestamp"`
	Name   string       `json:"name"`
	Labels MetricLabels `json:"labels"`
	Value  float64      `json:"value"`
}

type AggregatedMetric struct {
	Label string  `json:"label"` // Metric label as it was recorded or formatted with GroupMetricBy
	Value float64 `json:"value"`
}

type GroupMetricBy func(m Metric) string

// Query looks for matching metrics within the specified timeframe.
func (k *Kero) Query(metric string, labelFilters MetricLabels, start int64, end int64) ([]Metric, error) {
	q, err := k.db.Querier(start, end)
	if err != nil {
		return []Metric{}, err
	}
	defer q.Close()

	matchers := matchersForLabels(metric, labelFilters)
	if len(matchers) == 0 {
		catchAllMatcher, _ := plabels.NewMatcher(plabels.MatchRegexp, plabels.MetricName, ".*")
		matchers = append(matchers, catchAllMatcher)
	}
	ss := q.Select(context.Background(), true, nil, matchers...)
	var metrics []Metric
	for ss.Next() {
		match := ss.At()
		labels := match.Labels()
		it := match.Iterator(nil)
		for it.Next() == chunkenc.ValFloat {
			ts, val := it.At()
			metricLabels := labelsToMap(labels)
			metrics = append(metrics, Metric{ts, metricLabels[plabels.MetricName], metricLabels, val})
		}
	}

	sort.SliceStable(metrics, func(i, j int) bool { return metrics[i].Ts > metrics[j].Ts })

	return metrics, nil
}

// Count is an optimized version of AggregateDistinct counting occurrences of a metric in the specified timeframe.
func (k *Kero) Count(metric string, start int64, end int64) int {
	q, err := k.db.Querier(start, end)
	if err != nil {
		return 0
	}
	defer q.Close()

	matcher, _ := plabels.NewMatcher(plabels.MatchRegexp, plabels.MetricName, metric)
	ss := q.Select(context.Background(), true, nil, matcher)

	count := 0
	for ss.Next() {
		match := ss.At()
		it := match.Iterator(nil)
		for it.Next() == chunkenc.ValFloat {
			count += 1
		}
	}

	return count
}

// CountHistogram returns metric count within the specified timeframe for each time subdivision
// based on the duration between start and end time. Subdivisions are determined as follows:
//
//   - duration up to 3 days: 72 subdivisions each of 1 hour
//   - duration up to 31 days (ie. 1 month): 1 day
//   - duration up to 93 days (ie. 3 months): 1 week
//   - for durations longer than 3 months: 1 month
func (k *Kero) CountHistogram(metric string, start int64, end int64) [][2]int64 {
	aggUnit := selectTimeUnitForTimeframe(start, end)
	timeframes := timeSplits(aggUnit, start, end)
	counts := make([][2]int64, len(timeframes))

	for i, timeframe := range timeframes {
		count := k.Count(metric, timeframe[0], timeframe[1])
		counts[i] = [2]int64{timeframe[0], int64(count)}
	}

	return counts
}

// CountVisitors counts number of unique visitors for which an event with matching filters has been tracked
// within the specified timeframe.
func (k *Kero) CountVisitors(metric string, labelFilters MetricLabels, start int64, end int64) (int, error) {
	data, err := k.Query(metric, labelFilters, start, end)
	if err != nil {
		return 0, err
	}

	visitorIds := make(map[string]bool)
	for _, metric := range data {
		if id, exists := metric.Labels[VisitorIdLabel]; exists {
			if _, tracked := visitorIds[id]; !tracked {
				visitorIds[id] = true
			}
		}
	}

	return len(visitorIds), nil
}

// VisitorsHistogram returns a number of unique visitors per time subdivision in the specified timeframe.
// See [Kero.CountHistogram] for reference on time subdivisions.
func (k *Kero) VisitorsHistogram(metric string, filters MetricLabels, start int64, end int64) [][2]int64 {
	timeframes := timeSplits(selectTimeUnitForTimeframe(start, end), start, end)
	counts := make([][2]int64, len(timeframes))

	for i, timeframe := range timeframes {
		count, err := k.CountVisitors(metric, filters, timeframe[0], timeframe[1])
		if err != nil {
			count = 0
		}

		counts[i] = [2]int64{timeframe[0], int64(count)}
	}

	return counts
}

//go:generate stringer -type=AggregationMethod
type AggregationMethod int

const (
	AggregateCount AggregationMethod = iota // Aggregates by counting number of matched events
	AggregateSum                            // Aggregates by summing values of matched events
	AggregateAvg                            // Aggregates by calculating an average value of matched events
)

// AggregateDistinct provides advanced options to query the database.
// Data can be filtered using the metric name or any combination of labels (including negation).
// Additionally data can be grouped by a calculated key and aggregated using count, sum or average.
// Example:
//
//	 func QueryExample() {
//	   k, _ := kero.New(kero.WithDatabasePath("./kero"))
//	   data, _ := k.AggregateDistinct(
//	     "http_req", // get all "http_req" metrics
//	     func(m Metric) string { return m.Labels["$city"] }, // group them by city
//	     MetricLabels{ "country": "CH", "region!=": "ZH" } // filtering only requests coming from Switzerland, from any region except Zurich,
//	     kero.AggregateCount, // and return only the count of matched rows
//	     0, // from beginning of time
//	     time.Now().Unix(), // ...until now
//		  )
//
//	   fmt.Println("Found", len(data), "records:")
//	   for _, row := range data {
//	     fmt.Println(row.Value, "visitors from", row.Label)
//	   }
//	 }
//
// Results are sorted by highest value first.
func (k *Kero) AggregateDistinct(
	metricName string,
	groupBy GroupMetricBy,
	labelFilters MetricLabels,
	aggregateBy AggregationMethod,
	start int64,
	end int64,
) ([]AggregatedMetric, error) {
	counts := make(map[string]int)
	sums := make(map[string]float64)

	metrics, err := k.Query(metricName, labelFilters, start, end)
	if err != nil {
		return []AggregatedMetric{}, err
	}

	for _, metric := range metrics {
		id := groupBy(metric)
		if len(id) > 0 {
			counts[id] += 1

			if aggregateBy == AggregateSum || aggregateBy == AggregateAvg {
				sums[id] += metric.Value
			}
		}
	}

	allMetrics := []AggregatedMetric{}
	for id, value := range counts {
		var val float64
		switch aggregateBy {
		case AggregateCount:
			val = float64(value)
		case AggregateSum:
			val = sums[id]
		case AggregateAvg:
			val = sums[id] / float64(value)
		}

		allMetrics = append(allMetrics, AggregatedMetric{
			Label: id,
			Value: val,
		})
	}

	sort.SliceStable(allMetrics, func(i, j int) bool {
		return allMetrics[i].Value > allMetrics[j].Value
	})

	return allMetrics, nil
}

// CountDistinctByVisitor returns a number of unique visitors for which the matching events have been tracked.
func (k *Kero) CountDistinctByVisitor(
	metricName string,
	groupBy GroupMetricBy,
	labelFilters MetricLabels,
	start int64,
	end int64,
) ([]AggregatedMetric, error) {
	// { "group1": {"visitor1": true, "visitor2": true, ...}, ... }
	counts := make(map[string]map[string]bool)
	metrics, err := k.Query(metricName, labelFilters, start, end)
	if err != nil {
		return []AggregatedMetric{}, err
	}

	for _, metric := range metrics {
		if visitorId, ok := metric.Labels[VisitorIdLabel]; ok {
			if id := groupBy(metric); len(id) > 0 {
				if _, exists := counts[id]; !exists {
					counts[id] = make(map[string]bool)
				}
				if _, tracked := counts[id][visitorId]; !tracked {
					counts[id][visitorId] = true
				}
			}
		}
	}

	allMetrics := []AggregatedMetric{}
	for id, value := range counts {
		allMetrics = append(allMetrics, AggregatedMetric{
			Label: id,
			Value: float64(len(value)),
		})
	}

	sort.SliceStable(allMetrics, func(i, j int) bool { return allMetrics[i].Value > allMetrics[j].Value })

	return allMetrics, nil
}

// CountDistinctByVisitorAndLabel is a convenience method that's groups metrics simply by using the specified label.
// If filtering by [Kero.HttpRouteLabel], requests are grouped by both the HTTP method and the route, this way
// a distinction can be made between `GET /user/:id` and `POST /user/:id`.
// Events without the label itselfz are excluded from the count.
func (k *Kero) CountDistinctByVisitorAndLabel(
	metric string,
	label string,
	labelFilters MetricLabels,
	start int64,
	end int64,
) ([]AggregatedMetric, error) {
	if label == HttpRouteLabel {
		return k.CountDistinctByVisitor(metric, groupByRoute, labelFilters, start, end)
	}

	return k.CountDistinctByVisitor(metric, groupByLabel(label), labelFilters, start, end)
}

func groupByLabel(label string) GroupMetricBy {
	return func(m Metric) string {
		val := m.Labels[label]
		if len(val) == 0 {
			return ""
		}

		return val
	}
}

func groupByRoute(m Metric) string {
	method := m.Labels[HttpMethodLabel]
	route := m.Labels[HttpRouteLabel]
	if len(method) == 0 || len(route) == 0 {
		return ""
	}

	return strings.ToUpper(method) + " " + route
}

// (TODO) it should not silently ignore errors when creating matchers
func matchersForLabels(metric string, labels MetricLabels) []*plabels.Matcher {
	var matchers []*plabels.Matcher
	if len(metric) > 0 {
		if matcher, err := plabels.NewMatcher(plabels.MatchEqual, plabels.MetricName, metric); err == nil {
			matchers = append(matchers, matcher)
		}
	}

	for key, value := range labels {
		var matchType = plabels.MatchEqual
		if strings.HasSuffix(key, "!=") {
			matchType = plabels.MatchNotEqual
			key = strings.TrimSuffix(key, "!=")
		}

		if matcher, err := plabels.NewMatcher(matchType, key, value); err == nil {
			matchers = append(matchers, matcher)
		}
	}

	return matchers
}

func labelsToMap(labels plabels.Labels) MetricLabels {
	labelMap := make(MetricLabels)
	for _, label := range labels {
		labelMap[label.Name] = label.Value
	}
	return labelMap
}

const (
	AggregateByHour    time.Duration = time.Hour
	AggregateByDay                   = time.Hour * 24
	AggregateByWeek                  = time.Hour * 24 * 7
	AggregateByMonth                 = time.Hour * 24 * 31
	AggregateByQuarter               = time.Hour * 24 * 31 * 3
	AggregateByYear                  = time.Hour * 24 * 31 * 12
)

func selectTimeUnitForTimeframe(start, end int64) time.Duration {
	startTime := time.Unix(start, 0)
	endTime := time.Unix(end, 0)
	diff := endTime.Sub(startTime).Abs().Hours()

	// 3 days
	if diff <= 24*3 {
		return AggregateByHour
	}
	// 1 month
	if diff <= 24*31 {
		return AggregateByDay
	}
	// 3 months
	if diff <= 24*31*3 {
		return AggregateByWeek
	}

	// for time span bigger than 3 months
	return AggregateByMonth
}

func timeSplits(unit time.Duration, start int64, end int64) [][2]int64 {
	splits := [][2]int64{}
	increment := int64(unit.Seconds())

	for start < end {
		splits = append(splits, [2]int64{start, start + increment})
		start += increment
	}

	return splits
}
