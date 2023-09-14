package kero

import (
	"errors"
	"strings"
	"time"

	"github.com/oschwald/geoip2-golang"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/tsdb"
)

type Kero struct {
	dbPath                 string
	dbRetentionDuration    int64
	db                     *tsdb.DB
	geoDB                  *geoip2.Reader
	reverseLookupIP        bool
	DashboardPath          string
	PixelPath              string
	MeasureRequestDuration bool
	IgnoreCommonPaths      bool
	IgnoreBots             bool
	IgnoreDNT              bool
}

type MetricLabels map[string]string

const MetricName = labels.MetricName
const HttpReqMetricName = "http_req"
const HttpReqDurationMetricName = "http_req_dur"
const HttpMethodLabel = "$http_method"
const HttpPathLabel = "$http_path"
const HttpRouteLabel = "$http_route"
const BrowserNameLabel = "$browser_name"
const BrowserVersionLabel = "$browser_version"
const BrowserDeviceLabel = "$browser_device"
const BrowserOSLabel = "$browser_os"
const BrowserOSVersionLabel = "$browser_os_version"
const BrowserFormFactorLabel = "$browser_form_factor"
const ReferrerLabel = "$referrer"
const ReferrerDomainLabel = "$referrer_domain"
const UTMContentLabel = "$utm_content"
const UTMMediumLabel = "$utm_medium"
const UTMSourceLabel = "$utm_source"
const UTMCampaignLabel = "$utm_campaign"
const UTMTermLabel = "$utm_term"
const ClickIdGoogleLabel = "$clid_go"
const ClickIdFbLabel = "$clid_fb"
const ClickIdMsLabel = "$clid_ms"
const ClickIdTwLabel = "$clid_tw"
const CountryLabel = "$country"
const RegionLabel = "$region"
const CityLabel = "$city"
const IsBotLabel = "$is_bot"
const VisitorIdLabel = "$visitor_id"

type KeroOption func(*Kero) error

// New automatically creates a new Kero database on-disk if one doesn't exist already.
// See WithXXX functions for option configuration.
func New(options ...KeroOption) (*Kero, error) {
	k := &Kero{}

	for _, option := range options {
		if err := option(k); err != nil {
			return nil, err
		}
	}

	if len(k.dbPath) == 0 {
		return nil, errors.New("missing Kero database path")
	}
	tsdbOpts := tsdb.DefaultOptions()
	if k.dbRetentionDuration > 0 {
		tsdbOpts.RetentionDuration = k.dbRetentionDuration
	}
	db, err := tsdb.Open(k.dbPath, nil, nil, tsdbOpts, nil)
	if err != nil {
		return nil, err
	} else {
		k.db = db
	}

	if len(k.DashboardPath) == 0 {
		k.DashboardPath = "/_kero"
	}

	return k, nil
}

// WithDB sets the location of the database folder. Automatically created if it doesn't exist.
func WithDB(dbPath string) KeroOption {
	return func(k *Kero) error {
		k.dbPath = dbPath
		return nil
	}
}

// WithDashboardPath sets the URL at which the dashboard will be mounted. Defaults to `"/_kero"`.
func WithDashboardPath(path string) KeroOption {
	return func(k *Kero) error {
		if !isValidPathArg(path) {
			return errors.New("DashboardPath must start with / and have at least one more character")
		}
		k.DashboardPath = path
		return nil
	}
}

// WithGeoIPDB loads the MaxMind GeoLine2 and GeoIP2 database for IP-reverse lookup of visitors.
// If not provided, IP reverse-lookup is disabled.
func WithGeoIPDB(geoIPDBPath string) KeroOption {
	return func(k *Kero) error {
		if len(geoIPDBPath) == 0 {
			return errors.New("GeoIP database path is empty")
		}

		geoDB, err := geoip2.Open(geoIPDBPath)
		if err != nil {
			return err
		}
		k.geoDB = geoDB
		k.reverseLookupIP = true

		return nil
	}
}

// WithRequestMeasurements sets whether Kero should automatically measure response of handlers time.
func WithRequestMeasurements(value bool) KeroOption {
	return func(k *Kero) error {
		k.MeasureRequestDuration = value
		return nil
	}
}

// WithWebAssetsIgnored sets whether Kero should ignore requests made to common web assets.
//
// Ignored paths include:
//
//   - .css and .js files
//   - images (.png, .svg, .jpg, .webp, etc.)
//   - fonts (.woff2, .ttf, .otf, etc.)
//   - common URLs accessed by web scrapers (.php, .asp, etc.)
//   - any path starting with /., /_, /wp- and /public.
//
// Kero's resources and paths are always excluded.
func WithWebAssetsIgnored(value bool) KeroOption {
	return func(k *Kero) error {
		k.IgnoreCommonPaths = value
		return nil
	}
}

// WithBotsIgnored sets whether Kero should ignore requests made by bots, web scrapers and HTTP libraries.
// Bots and libraries are detected using their `User-Agent` header.
func WithBotsIgnored(value bool) KeroOption {
	return func(k *Kero) error {
		k.IgnoreBots = value
		return nil
	}
}

// WithDntIgnored sets whether Kero should ignore value of DNT header. If configured to ignore,
// requests with DNT: 1 (opt-out of tracking) will be still tracked.
func WithDntIgnored(value bool) KeroOption {
	return func(k *Kero) error {
		k.IgnoreDNT = value
		return nil
	}
}

// WithRetention defines the how long data points should be kept. Defaults to 15 days.
func WithRetention(duration time.Duration) KeroOption {
	return func(k *Kero) error {
		k.dbRetentionDuration = duration.Milliseconds()
		return nil
	}
}

// WithPixelPath defines the route at which the pixel tracker will be available to external applications.
// The pixel can be referenced from static websites or services not directly served by the Go server.
// Requests referer header will be used as the path, with other headers and query parameters used unchanged.
// Response of the pixel path will be a 1x1px transparent GIF.
func WithPixelPath(path string) KeroOption {
	return func(k *Kero) error {
		if !isValidPathArg(path) {
			return errors.New("PixelPath must start with / and have at least one more character")
		}
		k.PixelPath = path
		return nil
	}
}

func (k *Kero) Close() error {
	return k.db.Close()
}

func mergeMaps(maps ...map[string]string) map[string]string {
	result := make(map[string]string)
	for _, m := range maps {
		for k, v := range m {
			if len(v) > 0 {
				result[k] = v
			}
		}
	}
	return result
}

func isValidPathArg(path string) bool {
	return len(path) >= 2 && strings.HasPrefix(path, "/")
}
