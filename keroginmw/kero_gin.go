package keroginmw

import (
	"bytes"
	"fmt"
	"io/fs"
	"net/http"
	"net/url"

	"github.com/josip/kero"

	"github.com/gin-gonic/gin"
)

// Mount registers the dashboard UI, request tracking and the pixel tracker endpoints on the Gin server
func Mount(r *gin.Engine, k *kero.Kero, auth gin.Accounts) error {
	mountDashboard(r, k, auth)
	mountPixel(r, k)
	r.Use(requestTracker(k))
	return nil
}

// requestTracker is Gin middleware function that installs the request tracker.
func requestTracker(k *kero.Kero) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		if k.ShouldTrackHttpRequest(ctx.Request.URL.Path) {
			trackedHttpReq := kero.TrackedRequestFromHttp(ctx.Request)
			trackedHttpReq.Route = ctx.FullPath()
			k.TrackHttpRequest(trackedHttpReq)
			if k.MeasureRequestDuration {
				k.MeasureHttpRequest(trackedHttpReq, ctx.Next)
			} else {
				ctx.Next()
			}
		} else {
			ctx.Next()
		}
	}
}

// mountDashboard mounts the Kero dashboard interface.
// The path is specified using `WithDashboardPath` configuration option when creating the Kero instance.
func mountDashboard(r *gin.Engine, k *kero.Kero, accounts gin.Accounts) {
	assetsFs, _ := fs.Sub(kero.DashboardWebAssets, "assets")
	httpFS := http.FS(assetsFs)

	group := r.Group(k.DashboardPath, gin.BasicAuth(accounts))
	group.GET("", func(ctx *gin.Context) {
		ctx.Status(http.StatusOK)
		ctx.Header("Content-Type", "text/html;charset=utf-8")
		dash := kero.DefaultDashboard
		dash.LoadData(k, ctx.Query("t"))

		if err := dash.Write(ctx.Writer); err != nil {
			fmt.Println("[kero] error rendering template", err)
		}
	})
	group.StaticFS("assets", httpFS)
}

// mountPixel adds the pixel tracker to the Gin router.
func mountPixel(r *gin.Engine, k *kero.Kero) {
	if len(k.PixelPath) == 0 {
		return
	}

	r.GET(k.PixelPath, func(ctx *gin.Context) {
		if referrer, err := url.Parse(ctx.Request.Referer()); err == nil {
			if k.ShouldTrackHttpRequest(referrer.Path) {
				trackedHttpReq := kero.TrackedRequestFromHttp(ctx.Request)
				trackedHttpReq.Path = referrer.Path
				trackedHttpReq.Route = ""
				trackedHttpReq.Headers.Del("Referer")

				k.TrackHttpRequest(trackedHttpReq)
			}
		}

		headers := map[string]string{
			"Expires":       "Tue, 12 Sept 2023 06:00:00 GMT",
			"Cache-Control": "private, max-age=0, no-cache, must-revalidate, proxy-revalidate",
		}

		reader := bytes.NewReader(kero.Pixel)
		ctx.DataFromReader(http.StatusOK, kero.PixelSize, "image/gif", reader, headers)
	})
}
