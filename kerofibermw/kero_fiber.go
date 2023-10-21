package kerofibermw

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"

	"github.com/josip/kero"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/basicauth"
	"github.com/gofiber/fiber/v2/middleware/filesystem"
)

func Mount(app *fiber.App, k *kero.Kero, auth basicauth.Config) error {
	mountDashboard(app, k, auth)
	mountPixel(app, k)
	app.Use(requestTracker(k))

	return nil
}

// requestTracker is a Fiber middleware function that installs the request tracker.
func requestTracker(k *kero.Kero) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if k.ShouldTrackHttpRequest(c.Path()) {
			trackedHttpReq := trackedHttpReqFromCtx(c)
			k.TrackHttpRequest(trackedHttpReq)
			if k.MeasureRequestDuration {
				var err error
				k.MeasureHttpRequest(trackedHttpReq, func() { err = c.Next() })
				return err
			} else {
				return c.Next()
			}
		} else {
			return c.Next()
		}
	}
}

func trackedHttpReqFromCtx(c *fiber.Ctx) kero.TrackedHttpReq {
	return kero.TrackedHttpReq{
		Method:   c.Method(),
		Path:     c.Path(),
		Headers:  copyHeaders(c.GetReqHeaders()),
		ClientIp: c.IP(),
		Query:    copyQuery(c.Queries()),
		// (TODO) How to get the route in Fiber given: https://docs.gofiber.io/api/ctx#route
		// Route: c.Route().Path,
	}
}

func copyHeaders(h map[string][]string) http.Header {
	headers := http.Header{}

	for k, v := range h {
		headers.Add(k, v[0])
	}

	return headers
}

func copyQuery(queries map[string]string) url.Values {
	values := url.Values{}

	for k, v := range queries {
		values.Add(k, v)
	}

	return values
}

// MountDashboard mounts the Kero dashboard interface.
// The path is specified using `WithDashboardPath` configuration option when creating the Kero instance.
func mountDashboard(app *fiber.App, k *kero.Kero, auth basicauth.Config) {
	assetsFs, _ := fs.Sub(kero.DashboardWebAssets, "assets")
	httpFS := http.FS(assetsFs)

	group := app.Group(k.DashboardPath, basicauth.New(auth))
	group.Get("", func(c *fiber.Ctx) error {
		c.Status(http.StatusOK)
		c.Set("Content-Type", "text/html;charset=utf-8")
		dash := kero.DefaultDashboard
		dash.LoadData(k, c.Query("t"))

		var buf bytes.Buffer
		wr := io.Writer(&buf)

		if err := dash.Write(wr); err != nil {
			fmt.Println("[kero] error rendering template", err)
			return err
		}

		c.Write(buf.Bytes())
		return nil
	})
	group.Use("/assets", filesystem.New(filesystem.Config{
		Root:   httpFS,
		Browse: false,
	}))
}

// mountPixel adds the pixel tracker to the Fiber app.
func mountPixel(app *fiber.App, k *kero.Kero) {
	if len(k.PixelPath) == 0 {
		return
	}

	app.Get(k.PixelPath, func(c *fiber.Ctx) error {
		if referrer, err := url.Parse(c.Get("Referer")); err == nil {
			if k.ShouldTrackHttpRequest(referrer.Path) {
				trackedHttpReq := trackedHttpReqFromCtx(c)
				trackedHttpReq.Path = referrer.Path
				trackedHttpReq.Route = ""
				trackedHttpReq.Headers.Del("Referer")

				k.TrackHttpRequest(trackedHttpReq)
			}
		}

		c.Status(http.StatusOK)
		c.Set("Content-Type", "image/gif")
		c.Set("Expires", "Tue, 12 Sept 2023 06:00:00 GMT")
		c.Set("Cache-Control", "private, max-age=0, no-cache, must-revalidate, proxy-revalidate")
		return c.Send(kero.Pixel)
	})
}
