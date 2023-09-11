package kero

import (
	"embed"
	"errors"
	"fmt"
	"html/template"
	"io"
	"time"

	"github.com/josip/timewarp"
)

//go:embed index.html
var dashboardHtml string

//go:embed assets
var DashboardWebAssets embed.FS

var dashboardTemplate *template.Template
var dashboardTemplateErr error

func init() {
	dashboardTemplate, dashboardTemplateErr = template.New("index.html").Parse(dashboardHtml)

	if dashboardTemplateErr != nil {
		fmt.Println("kero dashboard template did not load:", dashboardTemplateErr)
	}
}

type Dashboard struct {
	Title      string
	ShowFooter bool
	BasePath   string

	VisitorsChartData []BarChartData
	VisitorsTrend     Trend

	ViewsChartData []BarChartData
	ViewsTrend     Trend
	Rows           [][]DashboardStat
}

type BarChartData struct {
	Timestamp int64
	Value     int64
	Percent   float64
}

func (bcd *BarChartData) FormattedTimestamp() string {
	t := time.Unix(bcd.Timestamp, 0)
	if t.Hour() == 0 {
		return t.Format("Jan 02, 2006")
	} else {
		return t.Format("Jan 02, 2006 15:04")
	}
}

type LabelFormatter func(AggregatedMetric) string

type DashboardStat struct {
	Title            string
	UnitDisplayLabel string
	CountLabel       string

	QueryMetric      string
	QueryLabel       string
	QueryGroupBy     GroupMetricBy
	QueryFilters     MetricLabels
	QueryByVisitor   bool
	QueryAggregateBy AggregationMethod
	QueryExcludeBots bool

	FormatLabel LabelFormatter

	Data []AggregatedMetric
}

var formFactorEmojis = map[string]string{
	FormFactorBot:     "🤖 Bot",
	FormFactorMobile:  "📱 Mobile",
	FormFactorTablet:  "💻 Tablet",
	FormFactorDesktop: "🖥️ Desktop",
}

var botFilter = MetricLabels{
	(BrowserFormFactorLabel + "!="): FormFactorBot,
}

var DefaultDashboard = Dashboard{
	Title:      "App stats",
	ShowFooter: true,
	Rows: [][]DashboardStat{
		{
			{
				Title:            "Top pages",
				UnitDisplayLabel: "Page",
				CountLabel:       "Visitors",

				QueryMetric:      HttpReqMetricName,
				QueryLabel:       HttpPathLabel,
				QueryByVisitor:   true,
				QueryExcludeBots: true,
			},
			{
				Title:            "Top referrals",
				UnitDisplayLabel: "Site",
				CountLabel:       "Visitors",

				QueryMetric:      HttpReqMetricName,
				QueryLabel:       ReferrerDomainLabel,
				QueryByVisitor:   true,
				QueryExcludeBots: true,
			},
			{
				Title:            "Top locations",
				UnitDisplayLabel: "Country",
				CountLabel:       "Visitors",

				QueryMetric:      HttpReqMetricName,
				QueryLabel:       CountryLabel,
				QueryByVisitor:   true,
				QueryExcludeBots: true,

				FormatLabel: func(am AggregatedMetric) string {
					cc := am.Label
					return string(0x1F1E6+rune(cc[0])-'A') + string(0x1F1E6+rune(cc[1])-'A') + " " + cc
				},
			},
		},

		// 		{
		// 			{
		// 				Title:            "Top UTM sources",
		// 				UnitDisplayLabel: "Source",
		// 				CountLabel:       "Visitors",
		//
		// 				QueryMetric:    HttpReqMetricName,
		// 				QueryLabel:     UTMSourceLabel,
		// 				QueryByVisitor: true,
		// 			},
		// 			{
		// 				Title:            "Top UTM mediums",
		// 				UnitDisplayLabel: "Medium",
		// 				CountLabel:       "Visitors",
		//
		// 				QueryMetric:    HttpReqMetricName,
		// 				QueryLabel:     UTMMediumLabel,
		// 				QueryByVisitor: true,
		// 			},
		// 			{
		// 				Title:            "Top UTM campaigns",
		// 				UnitDisplayLabel: "Campaign",
		// 				CountLabel:       "Visitors",
		//
		// 				QueryMetric:    HttpReqMetricName,
		// 				QueryLabel:     UTMCampaignLabel,
		// 				QueryByVisitor: true,
		// 			},
		// 		},

		{
			{
				Title:            "Top form factors",
				UnitDisplayLabel: "Form factor",
				CountLabel:       "Visitors",

				QueryMetric:    HttpReqMetricName,
				QueryLabel:     BrowserFormFactorLabel,
				QueryByVisitor: true,

				FormatLabel: func(am AggregatedMetric) string {
					if emoji, ok := formFactorEmojis[am.Label]; ok {
						return emoji
					}

					return am.Label
				},
			},
			{
				Title:            "Top browsers",
				UnitDisplayLabel: "Browser",
				CountLabel:       "Visitors",

				QueryMetric:      HttpReqMetricName,
				QueryLabel:       BrowserNameLabel,
				QueryByVisitor:   true,
				QueryExcludeBots: true,
			},

			{
				Title:            "Top operating systems",
				UnitDisplayLabel: "Operating system",
				CountLabel:       "Visitors",

				QueryMetric: HttpReqMetricName,
				QueryFilters: MetricLabels{
					(BrowserFormFactorLabel + "!="): FormFactorBot,
				},
				QueryLabel:       BrowserOSLabel,
				QueryByVisitor:   true,
				QueryExcludeBots: true,
			},
		},

		{
			{
				Title:            "Top routes",
				UnitDisplayLabel: "Route",
				CountLabel:       "Visitors",

				QueryMetric:    HttpReqMetricName,
				QueryLabel:     HttpRouteLabel,
				QueryByVisitor: true,
			},

			{
				Title:            "Slowest routes",
				UnitDisplayLabel: "Route",
				CountLabel:       "avg ms",

				QueryMetric:      HttpReqDurationMetricName,
				QueryGroupBy:     groupByRoute,
				QueryAggregateBy: AggregateAvg,
			},

			{
				Title:            "Top bots and libraries",
				UnitDisplayLabel: "Bot",
				CountLabel:       "Rqs",

				QueryMetric: HttpReqMetricName,
				QueryFilters: MetricLabels{
					BrowserFormFactorLabel: FormFactorBot,
				},
				QueryLabel: BrowserNameLabel,
			},
		},
	},
}

func (d *Dashboard) Write(wr io.Writer) error {
	if dashboardTemplateErr != nil {
		return errors.Join(errors.New("failed to parse dashboard template"), dashboardTemplateErr)
	}

	return dashboardTemplate.Execute(wr, d)
}

func (d *Dashboard) prepareChartData(rows [][2]int64) (int64, []BarChartData) {
	var chartData []BarChartData

	count := int64(0)
	max := int64(0)

	for _, row := range rows {
		count += row[1]
		if row[1] > max {
			max = row[1]
		}
	}

	for _, row := range rows {
		chartData = append(chartData, BarChartData{
			Timestamp: row[0],
			Value:     row[1],
			Percent:   (float64(row[1]) / float64(max) * 100),
		})
	}

	return count, chartData
}

func (d *Dashboard) loadDataForTimeframe(k *Kero, start, end int64) {
	// not 100% accurate
	prevPeriodStart := start - (end - start)

	visitors := k.VisitorsHistogram(HttpReqMetricName, botFilter, start, end)
	d.VisitorsTrend.CurrentValue, d.VisitorsChartData = d.prepareChartData(visitors)
	if prevCount, err := k.CountVisitors(HttpReqMetricName, botFilter, prevPeriodStart, start); err == nil {
		d.VisitorsTrend.PreviousValue = int64(prevCount)
	}

	views := k.CountHistogram(HttpReqMetricName, start, end)
	d.ViewsTrend.CurrentValue, d.ViewsChartData = d.prepareChartData(views)
	d.ViewsTrend.PreviousValue = int64(k.Count(HttpReqMetricName, prevPeriodStart, start))

	for i := range d.Rows {
		for j := range d.Rows[i] {
			if err := d.Rows[i][j].runQuery(k, start, end); err != nil {
				fmt.Println("Error while running dashboard query", d.Rows[i][j].Title, err)
			}
		}
	}
}

func (d *Dashboard) LoadData(k *Kero, timeframe string) {
	if len(timeframe) == 0 {
		timeframe = "t"
	}
	start, end := parseTimeframeString(timeframe)
	d.loadDataForTimeframe(k, start, end)
	// TODO this should be probably somewhere else it's needed here to build correct path
	// to .css and .js assets in the outputted HTML
	d.BasePath = k.DashboardPath
}

type Trend struct {
	CurrentValue  int64
	PreviousValue int64
}

func (t *Trend) PercentChange() float64 {
	return float64(t.CurrentValue-t.PreviousValue) / float64(t.PreviousValue) * 100
}

func parseTimeframeString(tf string) (int64, int64) {
	eod := timewarp.Now().EndOfDay().Time.Unix()

	switch tf {
	case "t":
		return timewarp.Today().Time.Unix(), eod
	case "24h":
		return timewarp.Now().SubHours(24).Time.Unix(), timewarp.Now().Time.Unix()
	case "7d":
		return timewarp.Today().SubDays(7).Time.Unix(), eod
	case "30d":
		return timewarp.Today().SubDays(30).Time.Unix(), eod
	case "12m":
		// actually from 1st of the month from 1y ago
		mth := timewarp.Now().Time
		return mth.AddDate(0, -12, -mth.Day()+1).Unix(), eod
	case "mtd":
		mtd := timewarp.Today().Time
		return mtd.AddDate(0, 0, -mtd.Day()+1).Unix(), eod
	case "ytd":
		ytd := timewarp.Today().Time
		return ytd.AddDate(0, -int(ytd.Month()-1), -ytd.Day()+1).Unix(), eod
	default:
		// defaults to "today"
		return timewarp.Now().Time.Unix(), eod
	}
}

func (s *DashboardStat) validate() error {
	if len(s.QueryMetric) == 0 {
		return errors.New("missing QueryMetric")
	}

	if len(s.QueryLabel) == 0 && s.QueryGroupBy == nil {
		return errors.New("missing QueryLabel or QueryGroupBy func")
	}

	return nil
}

func (s *DashboardStat) runQuery(k *Kero, start, end int64) error {
	if err := s.validate(); err != nil {
		return err
	}

	if s.QueryExcludeBots {
		if s.QueryFilters == nil {
			s.QueryFilters = botFilter
		} else {
			s.QueryFilters[BrowserFormFactorLabel+"!="] = FormFactorBot
		}
	}

	var err error
	if len(s.QueryLabel) > 0 {
		if s.QueryByVisitor {
			s.Data, err = k.CountDistinctByVisitorAndLabel(s.QueryMetric, s.QueryLabel, s.QueryFilters, start, end)
		} else {
			s.Data, err = k.AggregateDistinct(s.QueryMetric, groupByLabel(s.QueryLabel), s.QueryFilters, s.QueryAggregateBy, start, end)
		}
	} else if s.QueryGroupBy != nil {
		if s.QueryByVisitor {
			s.Data, err = k.CountDistinctByVisitor(s.QueryMetric, s.QueryGroupBy, s.QueryFilters, start, end)
		} else {
			s.Data, err = k.AggregateDistinct(s.QueryMetric, s.QueryGroupBy, s.QueryFilters, s.QueryAggregateBy, start, end)
		}
	}

	if err != nil {
		return err
	}

	if s.FormatLabel != nil {
		for i, row := range s.Data {
			s.Data[i].Label = s.FormatLabel(row)
		}
	}

	return nil
}
