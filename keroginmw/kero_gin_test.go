package keroginmw_test

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/josip/kero"
	ktest "github.com/josip/kero/internal/kerotest"
	keromw "github.com/josip/kero/keroginmw"
)

func createServer(t *testing.T) (*gin.Engine, *kero.Kero) {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()

	k, _ := kero.New(
		kero.WithDB(t.TempDir()),
		kero.WithDashboardPath(ktest.DashPath),
		kero.WithWebAssetsIgnored(true),
		kero.WithRequestMeasurements(true),
		kero.WithBotsIgnored(false),
	)

	r.Use(keromw.RequestTracker(k))
	keromw.MountDashboard(r, k, gin.Accounts{
		ktest.DashUsername: ktest.DashPass,
	})

	r.GET(ktest.HelloPath, func(c *gin.Context) {
		c.String(200, "Hello "+c.Param("id"))
	})

	r.GET(ktest.WaitPath, func(c *gin.Context) {
		time.Sleep(ktest.WaitDuration)
		c.String(200, "Done waiting")
	})

	return r, k
}

func TestMountDashboard(t *testing.T) {
	r, k := createServer(t)
	defer k.Close()
	for _, test := range ktest.DashboardTests {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, test.Request())

		if test.HasFailed(w.Code) {
			t.Error(test.Description, ", response code was: ", w.Code)
		}
	}
}

func TestRequestTracker(t *testing.T) {
	r, k := createServer(t)
	defer k.Close()

	for _, test := range ktest.TrackingTests {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, test.Request())
	}

	ktest.ExpectRequestsTracked(t, k)
}

func TestMeasureDuration(t *testing.T) {
	r, k := createServer(t)
	defer k.Close()

	w := httptest.NewRecorder()
	r.ServeHTTP(w, ktest.WaitRequest)

	ktest.ExpectDurationTracked(t, k)
}
