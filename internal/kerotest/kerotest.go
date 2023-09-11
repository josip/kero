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
