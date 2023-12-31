package kerotest

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/josip/kero"
)

const DashPath = "/_kero_test"
const HelloPath = "/hello/:id"
const WaitPath = "/wait"
const WaitDuration = time.Duration(123 * time.Millisecond)
const DashUsername = "admin"
const DashPass = "pass"
const PixelPath = "/px.gif"
const pixelReferrerPath = "/blog/hello-mars"
const pixelReferrer = "http://localhost:1234" + pixelReferrerPath
const PrefixToIgnore = "/hello"

var WaitRequest = httptest.NewRequest("GET", WaitPath, nil)

type DashboardTest struct {
	Description string
	Path        string
	Authed      bool
	ExpectError bool
}

func (t *DashboardTest) Request() *http.Request {
	req := httptest.NewRequest("GET", t.Path, nil)
	if t.Authed {
		req.SetBasicAuth(DashUsername, DashPass)
	}
	return req
}

func (t *DashboardTest) HasFailed(statusCode int) bool {
	reqFailed := statusCode >= 400
	return (t.ExpectError && !reqFailed) || (!t.ExpectError && reqFailed)
}

var DashboardTests = []DashboardTest{
	{
		Description: "prevent unauthorized access",
		Path:        DashPath,
		Authed:      false,
		// gin returns 404, fiber 401 on invalid auth
		ExpectError: true,
	},
	{
		Description: "load CSS assets",
		Path:        DashPath + "/assets/css/app.css",
		Authed:      true,
		ExpectError: false,
	},
	{
		Description: "load dashboard with some content",
		Path:        DashPath,
		Authed:      true,
		ExpectError: false,
	},
}

type TrackingTest struct {
	Path              string
	ExpectToBeTracked bool
}

func (t *TrackingTest) Request() *http.Request {
	return httptest.NewRequest("GET", t.Path, nil)
}

var TrackingTests = []TrackingTest{
	{
		Path:              "/favicon.ico",
		ExpectToBeTracked: false,
	},
	{
		Path:              DashPath,
		ExpectToBeTracked: false,
	},
	{
		Path:              "/hello/world",
		ExpectToBeTracked: true,
	},
	{
		Path:              "/hello/earth",
		ExpectToBeTracked: true,
	},
	{
		Path:              "/hello/go",
		ExpectToBeTracked: true,
	},
	{
		Path:              "/",
		ExpectToBeTracked: true,
	},
}

func ExpectRequestsTracked(t *testing.T, k *kero.Kero) {
	expected := 0
	for _, req := range TrackingTests {
		if req.ExpectToBeTracked {
			expected += 1
		}
	}

	tracked := k.Count(kero.HttpReqMetricName, 0, time.Now().Unix())

	if tracked != expected {
		t.Error("expected", expected, "requests to be tracked, got", tracked)
	}
}

func ExpectDurationTracked(t *testing.T, k *kero.Kero) {
	wants := 1

	records, err := k.Query(kero.HttpReqDurationMetricName, kero.MetricLabels{}, 0, time.Now().Unix())
	if err != nil {
		t.Fatal("failed to query kero:", err)
	}

	if len(records) != wants {
		t.Fatal("expected", wants, "got", len(records), "tracked events")
	}

	if records[0].Value < float64(WaitDuration.Milliseconds()) {
		t.Error("expected request to take at least 100ms, got", records[0].Value)
	}
}

func PixelRequest() *http.Request {
	req := httptest.NewRequest("GET", PixelPath, nil)
	req.Header.Add("Referer", pixelReferrer)
	return req
}

func ExpectPixelToTrack(t *testing.T, k *kero.Kero) {
	res, err := k.Query(
		kero.HttpReqMetricName,
		kero.MetricLabels{
			kero.HttpPathLabel: pixelReferrerPath,
		},
		0,
		time.Now().Unix(),
	)

	if err != nil {
		t.Fatal("failed to query db", err)
	}

	if len(res) != 1 {
		t.Fatal("expected pixel to track paths but it did not")
	}

	res, err = k.Query(
		kero.HttpReqMetricName,
		kero.MetricLabels{
			kero.HttpPathLabel: PixelPath,
		},
		0,
		time.Now().Unix(),
	)
	if err != nil {
		t.Fatal("failed to query db", err)
	}

	if len(res) != 0 {
		t.Fatal("expected request to pixel path not to be tracked")
	}
}

func IgnoredHelloRequest() *http.Request {
	req := httptest.NewRequest("GET", "/hello/mars", nil)
	return req
}

func ExpectHelloIgnored(t *testing.T, k *kero.Kero) {
	tracked := k.Count(kero.HttpReqMetricName, 0, time.Now().Unix())
	if tracked != 0 {
		t.Fatal("expected request to ignore prefix not to be tracked")
	}
}
