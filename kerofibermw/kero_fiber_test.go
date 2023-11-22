package kerofibermw_test

import (
	"image"
	"testing"
	"time"

	_ "image/gif"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/basicauth"
	"github.com/josip/kero"
	ktest "github.com/josip/kero/internal/kerotest"
	keromw "github.com/josip/kero/kerofibermw"
)

func createServer(t *testing.T) (*fiber.App, *kero.Kero) {
	app := fiber.New()
	k, _ := kero.New(
		kero.WithDB(t.TempDir()),
		kero.WithDashboardPath(ktest.DashPath),
		kero.WithWebAssetsIgnored(true),
		kero.WithRequestMeasurements(true),
		kero.WithBotsIgnored(false),
		kero.WithPixelPath(ktest.PixelPath),
	)

	keromw.Mount(app, k, basicauth.Config{
		Users: map[string]string{
			ktest.DashUsername: ktest.DashPass,
		},
	})

	app.Get(ktest.HelloPath, func(c *fiber.Ctx) error {
		return c.SendString("Hello " + c.Params("id", "n/a"))
	})

	app.Get(ktest.WaitPath, func(c *fiber.Ctx) error {
		time.Sleep(ktest.WaitDuration)
		return c.SendString("Done waiting")
	})

	return app, k
}

func TestMountDashboard(t *testing.T) {
	app, k := createServer(t)
	defer k.Close()

	for _, test := range ktest.DashboardTests {
		resp, err := app.Test(test.Request())
		if err != nil {
			t.Error("request failed", test.Path, ":", err)
		}

		if test.HasFailed(resp.StatusCode) {
			t.Error(test.Description, ", response code was: ", resp.StatusCode)
		}
	}
}

func TestRequestTracker(t *testing.T) {
	app, k := createServer(t)
	defer k.Close()

	for _, test := range ktest.TrackingTests {
		if _, err := app.Test(test.Request()); err != nil {
			t.Fatal("request failed", test.Path, err)
		}
	}

	ktest.ExpectRequestsTracked(t, k)
}

func TestMeasureDuration(t *testing.T) {
	app, k := createServer(t)
	defer k.Close()

	if _, err := app.Test(ktest.WaitRequest); err != nil {
		t.Fatal("request failed", err)
	}

	ktest.ExpectDurationTracked(t, k)
}

func TestPixel(t *testing.T) {
	app, k := createServer(t)
	defer k.Close()

	if resp, err := app.Test(ktest.PixelRequest()); err != nil {
		t.Fatal("request failed", err)
	} else {
		if _, format, err := image.DecodeConfig(resp.Body); format != "gif" || err != nil {
			t.Error("pixel was not a valid gif file", format, err)
		}

		ktest.ExpectPixelToTrack(t, k)
	}
}

func TestIgnoreCustomPath(t *testing.T) {
	app, k := createServer(t)
	defer k.Close()

	k.IgnoredPrefixes = append(k.IgnoredPrefixes, ktest.PrefixToIgnore)
	if _, err := app.Test(ktest.IgnoredHelloRequest()); err != nil {
		t.Fatal("request failed", err)
	} else {
		ktest.ExpectHelloIgnored(t, k)
	}
}
