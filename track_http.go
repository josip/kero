package kero

import (
	"crypto/md5"
	"encoding/hex"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/mileusna/useragent"
)

var commonAssetPrefixes = []string{
	"/.",
	"/_",
	// various bad bots testing for wordpress
	"/wp-",
	"/public",
}

var commonAssetSuffixes = []string{
	".js",
	".css",
	".png",
	".jpg",
	".jpeg",
	".webp",
	".gif",
	".svg",
	".woff",
	".woff2",
	".otf",
	".tff",
	".ico",
	".mov",
	".mpg",
	".mpg3",
	".mpg4",
	".wav",
	".ogg",
	// various bad bots
	".php",
	".asp",
	".aspx",
}

type TrackedHttpReq struct {
	Method     string
	Path       string
	Headers    http.Header
	Query      url.Values
	Route      string
	ClientIp   string
	RemoteAddr string
}

func TrackedRequestFromHttp(httpReq *http.Request) TrackedHttpReq {
	return TrackedHttpReq{
		Method:     httpReq.Method,
		Path:       httpReq.URL.Path,
		Headers:    httpReq.Header,
		Query:      httpReq.URL.Query(),
		RemoteAddr: httpReq.RemoteAddr,
	}
}

func (k *Kero) ShouldTrackHttpRequest(path string) bool {
	if strings.HasPrefix(path, k.DashboardPath) {
		return false
	}

	if k.IgnoreCommonPaths {
		if path == "/favicon.ico" {
			return false
		}

		for _, prefix := range commonAssetPrefixes {
			if strings.HasPrefix(path, prefix) {
				return false
			}
		}

		for _, suffix := range commonAssetSuffixes {
			if strings.HasSuffix(path, suffix) {
				return false
			}
		}
	}

	return true
}

func (k *Kero) TrackWithRequest(metric string, labels MetricLabels, value float64, req TrackedHttpReq) error {
	if !k.IgnoreDNT && req.Headers.Get("DNT") == "1" {
		return nil
	}

	clientIp := req.ClientIp
	if len(clientIp) == 0 {
		clientIp = getClientIp(req.Headers, req.RemoteAddr)
	}

	allLabels := mergeMaps(
		labels,
		MetricLabels{
			HttpMethodLabel: req.Method,
			HttpPathLabel:   req.Path,
			HttpRouteLabel:  req.Route,
		},
		k.visitorId(clientIp, req.Headers),
		k.locationLabels(clientIp),
		k.userAgentLabels(req.Headers),
		k.referrerLabels(req.Headers),
		k.utmLabels(req.Query),
	)
	if k.IgnoreBots && allLabels[BrowserFormFactorLabel] == FormFactorBot {
		return nil
	}

	return k.TrackOne(metric, allLabels)
}

func (k *Kero) TrackOneWithRequest(metric string, labels MetricLabels, req TrackedHttpReq) error {
	return k.TrackWithRequest(metric, labels, 1, req)
}

func (k *Kero) TrackHttpRequest(req TrackedHttpReq) error {
	return k.TrackOneWithRequest(HttpReqMetricName, nil, req)
}

func (k *Kero) MeasureHttpRequest(req TrackedHttpReq, handler func()) {
	start := time.Now()

	defer func() {
		duration := time.Since(start)
		// duration.Milliseconds() performs integer rounding
		ds := float64(duration.Nanoseconds()) / float64(1e6)

		labels := MetricLabels{
			HttpMethodLabel: req.Method,
			HttpPathLabel:   req.Path,
			HttpRouteLabel:  req.Route,
			// "$status_code":  strconv.Itoa(req.HttpRequest.Response.StatusCode),
		}
		// fmt.Println("tracked", labels[HttpRouteLabel], labels["$status_code"], )

		k.Track(HttpReqDurationMetricName, labels, ds)
	}()

	handler()
}

func (k *Kero) visitorId(ip string, headers http.Header) MetricLabels {
	id := strings.Join([]string{
		ip,
		headers.Get("user-agent"),
		headers.Get("accept"),
		headers.Get("accept-encoding"),
		headers.Get("accept-language"),
	}, "|")
	hash := md5.Sum([]byte(id))
	hashString := hex.EncodeToString(hash[:])

	return MetricLabels{
		VisitorIdLabel: hashString,
	}
}

// Building labels for requests
func (k *Kero) locationLabels(clientIp string) MetricLabels {
	if !k.reverseLookupIP {
		return MetricLabels{}
	}

	var country, region, city string
	if ip := net.ParseIP(clientIp); ip != nil {
		if loc, err := k.geoDB.City(ip); err == nil {
			country = loc.Country.IsoCode
			city = loc.City.Names["en"]
			if subdivs := loc.Subdivisions; len(subdivs) > 0 {
				region = subdivs[0].Names["en"]
			}
		}
	}

	return MetricLabels{
		CountryLabel: country,
		RegionLabel:  region,
		CityLabel:    city,
	}
}

const FormFactorDesktop = "desktop"
const FormFactorMobile = "mobile"
const FormFactorTablet = "tablet"
const FormFactorBot = "bot"

func (k *Kero) userAgentLabels(headers http.Header) MetricLabels {
	uaString := headers.Get("user-agent")
	ua := useragent.Parse(uaString)

	var formFactor string
	if ua.Desktop {
		formFactor = FormFactorDesktop
	}
	if ua.Mobile {
		formFactor = FormFactorMobile
	}
	if ua.Tablet {
		formFactor = FormFactorTablet
	}
	if ua.Bot || isHttpClientLibrary(uaString) {
		formFactor = FormFactorBot
	}

	return MetricLabels{
		BrowserNameLabel:       ua.Name,
		BrowserVersionLabel:    ua.Version,
		BrowserDeviceLabel:     ua.Device,
		BrowserOSLabel:         ua.OS,
		BrowserOSVersionLabel:  ua.OSVersion,
		BrowserFormFactorLabel: formFactor,
	}
}

func (k *Kero) referrerLabels(headers http.Header) MetricLabels {
	var referrerHost string
	referrer := headers.Get("referer")
	if parsedUrl, err := url.Parse(referrer); err == nil {
		referrerHost = parsedUrl.Hostname()
	}

	return MetricLabels{
		ReferrerLabel:       referrer,
		ReferrerDomainLabel: referrerHost,
	}
}

func (k *Kero) utmLabels(queryParams url.Values) MetricLabels {
	return MetricLabels{
		UTMContentLabel:    queryParams.Get("utm_content"),
		UTMMediumLabel:     queryParams.Get("utm_medium"),
		UTMSourceLabel:     queryParams.Get("utm_source"),
		UTMCampaignLabel:   queryParams.Get("utm_campaign"),
		UTMTermLabel:       queryParams.Get("utm_term"),
		ClickIdGoogleLabel: queryParams.Get("gclid"),
		ClickIdFbLabel:     queryParams.Get("fbclid"),
		ClickIdMsLabel:     queryParams.Get("msclkid"),
		ClickIdTwLabel:     queryParams.Get("twclid"),
	}
}

var commonHttpClientLibraries = []string{
	// go
	"go-http-client",
	"github.com/monaco-io",
	"gentleman",
	// node.js
	"node-fetch",
	"undici",
	"axios",
	// objective-c + swift
	"alamofire",
	"nsurlconnection",
	"nsurlsession",
	"urlsession",
	"swifthttp",
	// python
	"python-", //-urlib3, -requests
	// java
	"apache-httpclient",
	// php requests
	"php-",
	"zend",
	"laminas",
	"guzzlehttp",
	// c#/.net todo
	// C/c++ todo
	// apps
	"curl",
	"wget",
	"rapidapi",
	"postman",
	// Apple App Site Association
	"aasa",
	// RSS readers
	"linkship",
	"feedbin",
	"feedly",
	"artykul",
	// others
	"x11",
	// render.com health check
	"render",
	"dataprovider.com",
	"researchscan",
	"zgrab",
	"NetcraftSurveyAgent",
}

func isHttpClientLibrary(ua string) bool {
	if len(ua) == 0 {
		return true
	}

	ua = strings.ToLower(ua)
	for _, clientName := range commonHttpClientLibraries {
		if strings.HasPrefix(ua, clientName) {
			return true
		}
	}

	return false
}

func getClientIp(headers http.Header, remoteAddr string) string {
	if ip := headers.Get("CF-Connecting-IP"); len(ip) > 1 {
		return ip
	}

	if ff := headers.Get("X-Forwarded-For"); len(ff) > 1 {
		if ips := strings.Split(ff, ", "); len(ips) > 0 {
			return ips[0]
		}
	}

	if ip := headers.Get("X-Real-IP"); len(ip) > 1 {
		return ip
	}

	if host, _, err := net.SplitHostPort(remoteAddr); err == nil {
		return host
	}

	return ""
}
