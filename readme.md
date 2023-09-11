# Kero 📊

Kero is a privacy-friendly, embeddable, analytics dashboard for your Go websites. With its drop-in integrations, it's the easiest way to get an overview of the key web metrics.

![Screenshot of a Kero dashboard](screenshot.png)

* **Privacy-friendly**: Kero tracks server-side requests with limited access to identifiable user data, compared to client-side solutions.
* **Embedded**: Import Kero middleware and you're ready to go, there are no additional databases or servers to provision and maintain.
* **Easy to understand**: Kero comes with a glanceable dashboard that contains all the data you care about on a single page.

## Getting started

Kero integrates with Gin and Fiber out of the box:

<details>
<summary><b>Gin example</b></summary>

```golang
package mywebsite

import (
    "os"

    "github.com/gin-gonic/gin"

    "github.com/josip/kero"
    keromw "github.com/josip/kero/keroginmw"
)

func Main() {
    r := gin.New()
    // 1) Initialize Kero
    k, _ := kero.New(
        kero.WithDBPath("./kero-stats"),
        kero.WithDashboardPath("/_kero"),
        // measures response time
        kero.WithRequestMeasurements(true),
        // doesn't log requests to .css/.js/.png/etc.
        kero.WithWebAssetsIgnored(true),
        // doesn't log requests from bots and HTTP libraries
        kero.WithBotsIgnored(true),
    )
    defer k.Close()

    // 2) Expose dashboard UI, protected with Basic Auth
    keromw.MountDashboard(r, k, gin.Accounts{
        os.Getenv("KERO_ADMIN_USERNAME"): os.Getenv("KERO_ADMIN_PW"),
    })

    // 3) Track all incoming requests
    r.Use(keromw.RequestTracker(k))

    r.GET("/hello", func(ctx *gin.Context) {
        c.String(200, "Hello")
    })

    r.Run()

    // 4) Open http://localhost:8080/_kero
}
```
</details>

<details>
<summary><b>Fiber example</b></summary>

```golang
package mywebsite

import (
    "os"

    "github.com/gofiber/fiber/v2"
    "github.com/gofiber/fiber/v2/middleware/basicauth"

    "github.com/josip/kero"
    keromw "github.com/josip/kero/kerofibermw"
)

func Main() {
    app := fiber.New()
    // 1) Initialize Kero
    k, _ := kero.New(
        kero.WithDBPath("./kero-stats"),
        kero.WithDashboardPath("/_kero"),
        // measures response time
        kero.WithRequestMeasurements(true),
        // doesn't log requests to .css/.js/.png/etc.
        kero.WithWebAssetsIgnored(true),
        // doesn't log requests from bots and HTTP libraries
        kero.WithBotsIgnored(true),
    )
    defer k.Close()

    // 2) Expose dashboard UI, protected with Basic Auth
    keromw.MountDashboard(app, k, basicauth.Config{
        Users: {
            os.Getenv("KERO_ADMIN_USERNAME"): os.Getenv("KERO_ADMIN_PW"),
        },
    })

    // 3) Track all incoming requests
    app.Use(keromw.RequestTracker(k))

    app.Get("/hello", func(ctx *fiber.Ctx) error {
        return ctx.SendString("Hello")
    })

    app.Listen(":8080")

    // 4) Open http://localhost:8080/_kero
}
```
</details>

Want to see support for other HTTP frameworks? [Create a ticket](https://github.com/josip/kero/issues/new) or submit a PR :octocat:.

## Tracked visitor data

Availability and accuracy of the data collected varies and should be considered as best-effort since browsers themselves and user-installed extensions can introduce noisy data.

* Browser name and its version
* OS name and its version
* Device name
* Device form factor (phone, tablet, desktop, bot)
* Referrer (based on HTTP header) and [UTM](https://en.wikipedia.org/wiki/UTM_parameters) query parameters
* Country, region and city based on user's IP address (disabled by default)
* Visitor ID (see below)

IP addresses and full `User-Agent` strings are nor stored nor logged in any manner by Kero. However other middleware or logging libraries might do so.

## How are visitors counted?

Each visitor is assigned a hashed ID that encodes their IP address and values of `Accept`, `Accept-Encoding`, `Accept-Language` and `User-Agent` HTTP headers.

As these values are not guaranteed to be unique even between consecutive visits of the same user they have been selected to provide an approximate data, while at the same time respecting user privacy as much as possible.

Value of the DNT header might be ignored.

## Tracked request metadata

* Request duration, if enabled
* Route (ie. `/user/:id` vs `/user/kero`; not tracked with Fiber)
* Distinction whether request was made by a web browser or programatically via different HTTP client libraries

## Displayed data

* Number of visitors in current and previous period (for comparison)
* Number of views (requests) in current and previous period
* Top pages
* Top referrals
* Top locations
* Top form factors
* Top browsers
* Top operating systems
* Top routes
* Slowest routes (not tracked with Fiber)
* Top bots and HTTP libraries

## Data storage

Kero is using [TSDB](https://pkg.go.dev/github.com/prometheus/tsdb) from Prometheus to store the data on the disk.

## Who's using Kero

* [Linkship](https://linkship.app)
