package keroginmw

import (
	"fmt"
	"io/fs"
	"net/http"

	"github.com/josip/kero"

	"github.com/gin-gonic/gin"
)

// RequestTracker is Gin middleware function that installs the request tracker.
func RequestTracker(k *kero.Kero) gin.HandlerFunc {
	return func(c *gin.Context) {
		if k.ShouldTrackHttpRequest(c.Request.URL.Path) {
			trackedHttpReq := kero.TrackedRequestFromHttp(c.Request)
			trackedHttpReq.Route = c.FullPath()
			k.TrackHttpRequest(trackedHttpReq)
			if k.MeasureRequestDuration {
				k.MeasureHttpRequest(trackedHttpReq, c.Next)
			} else {
				c.Next()
			}
		} else {
			c.Next()
		}
	}
}

// MountDashboard mounts the Kero dashboard interface.
// The path is specified using `WithDashboardPath` configuration option when creating the Kero instance.
func MountDashboard(r *gin.Engine, k *kero.Kero, accounts gin.Accounts) {
	assetsFs, _ := fs.Sub(kero.DashboardWebAssets, "assets")
	httpFS := http.FS(assetsFs)

	group := r.Group(k.DashboardPath, gin.BasicAuth(accounts))
	group.GET("", func(c *gin.Context) {
		c.Status(http.StatusOK)
		c.Header("Content-Type", "text/html;charset=utf-8")
		dash := kero.DefaultDashboard
		dash.LoadData(k, c.Query("t"))

		if err := dash.Write(c.Writer); err != nil {
			fmt.Println("[kero] error rendering template", err)
		}
	})
	group.StaticFS("assets", httpFS)
}
