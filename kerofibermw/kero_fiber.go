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

// RequestTracker is Gin middleware function that installs the request tracker.
func RequestTracker(k *kero.Kero) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if k.ShouldTrackHttpRequest(c.Path()) {
			trackedHttpReq := kero.TrackedHttpReq{
				Method:   c.Method(),
				Path:     c.Path(),
				Headers:  copyHeaders(c.GetReqHeaders()),
				ClientIp: c.IP(),
				Query:    copyQuery(c.Queries()),
				// INNACURATE https://docs.gofiber.io/api/ctx#route
				// Route: c.Route().Path,
			}

			k.TrackHttpRequest(trackedHttpReq)
			if k.MeasureRequestDuration {
				// TODO swallows errors
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

func copyHeaders(h map[string]string) http.Header {
	headers := http.Header{}

	for k, v := range h {
		headers.Add(k, v)
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
func MountDashboard(app *fiber.App, k *kero.Kero, auth basicauth.Config) {
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
