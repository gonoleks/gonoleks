<div align="center">

<a name="readme-top"></a>

<a href="https://github.com/gonoleks/gonoleks">
  <img width="450px" alt="Gonoleks" src="assets/gonoleks-logo.png">
</a>

# Gonoleks

🐦‍⬛🐦‍⬛🐦‍⬛ **Gonoleks** is a modern, high-performance **micro-framework** written in [Go][go_url].<br/>It features a [Gin][gin_url] inspired API with lightning-faster **performance**.<br/>Built on [fasthttp][fasthttp_url], the **fastest** HTTP engine for Go.

[![Go version][go_version_img]][go_dev_url]
[![Go report][go_report_img]][go_report_url]
[![GitHub Release][repo_release_img]][repo_url]
[![GitHub License][repo_license_img]][repo_license_url]

</div>

## ✨ Features

- Secure
- Gin-inspired API
- Memory Efficient
- Lightweight
- Fast, fast, fast!

## ⚙️ Installation

> [!IMPORTANT]
> Gonoleks requires **Go version `1.23` or higher** to run.

Install Gonoleks using `go get`:

```sh
go get -u github.com/gonoleks/gonoleks
```

## ⚡️ Quick Start

A simple "Hello, World!" web server:

```go
package main

import (
    "github.com/gonoleks/gonoleks"
)

func main() {
    // Initialize a default Gonoleks app
    app := gonoleks.Default()

    // Define a route for the GET method on the root path '/'
    app.GET("/", func(c *gonoleks.Context) {
        // Send a string response with status code 200
        c.String(200, "Hello, World!")
    })

    // Start the server on port 3000
    app.Run(":3000")
}
```

Visit `http://localhost:3000` to see your app in action.

<div align="right">

[&nwarr; Back to top](#readme-top)

</div>

## 📖 Examples

Listed below are some of the common examples. More available in our [Examples Repository][examples_url].

### Route Parameters

Gonoleks provides a powerful routing system with support for various parameter types:

```go
package main

import (
    "github.com/gonoleks/gonoleks"
)

func main() {
    app := gonoleks.Default()

    // GET /api/anything
    app.GET("/api/*", func(c *gonoleks.Context) {
        c.String(200, "✋🏻 %s", c.Param("*"))
        // => ✋🏻 anything
    })

    // GET /missions/earth-mars
    app.GET("/missions/:from-:to", func(c *gonoleks.Context) {
        c.String(200, "🚀 From: %s, To: %s", c.Param("from"), c.Param("to"))
        // => 🚀 From: earth, To: mars
    })

    // GET /readme.md
    app.GET("/:file.:ext", func(c *gonoleks.Context) {
        c.String(200, "📄 %s.%s", c.Param("file"), c.Param("ext"))
        // => 📄 readme.md
    })

    // GET /john/33
    app.GET("/:name/:age", func(c *gonoleks.Context) {
        c.String(200, "👨🏻 %s is %s years old", c.Param("name"), c.Param("age"))
        // => 👨🏻 john is 33 years old
    })

    // GET /john
    app.GET("/:name", func(c *gonoleks.Context) {
        c.String(200, "👋🏻 Hello, %s", c.Param("name"))
        // => 👋🏻 Hello john
    })

    app.Run(":3000")
}
```

<div align="right">

[&nwarr; Back to top](#readme-top)

</div>

### Route Groups

Organize your routes with logical grouping:

```go
package main

import (
    "github.com/gonoleks/gonoleks"
)

func main() {
    app := gonoleks.Default()

    // Create a group for API endpoints
    api := app.Group("/api")

    // Create a nested group for user-related endpoints
    users := api.Group("/users")

    // GET /api/users
    users.GET("/", func(c *gonoleks.Context) {
        c.JSON(200, gonoleks.H{
            "users": []string{"James", "Maria", "Angela", "Eddie", "Laura"},
        })
    })

    // GET /api/users/:id
    users.GET("/:id", func(c *gonoleks.Context) {
        c.JSON(200, gonoleks.H{
            "id":   c.Param("id"),
            "name": "User #" + c.Param("id"),
        })
    })

    // Create another group for location-related endpoints
    locations := api.Group("/locations")

    // GET /api/locations
    locations.GET("/", func(c *gonoleks.Context) {
        c.JSON(200, gonoleks.H{
            "locations": []string{"Toluca Lake", "Brookhaven Hospital", "Lakeview Hotel"},
        })
    })

    app.Run(":3000")
}
```

<div align="right">

[&nwarr; Back to top](#readme-top)

</div>

### Static Files

Serving static files is straightforward with Gonoleks:

```go
package main

import (
    "github.com/gonoleks/gonoleks"
)

func main() {
    app := gonoleks.Default()

    // Serve static files from the "./assets" directory
    app.Static("/*", "./assets")
    // => http://localhost:3000/js/script.js
    // => http://localhost:3000/css/style.css

    app.Static("/static", "./assets")
    // => http://localhost:3000/static/js/script.js
    // => http://localhost:3000/static/css/style.css

    // Serve a single file for any unmatched routes
    app.Static("*", "./assets/index.html")
    // => http://localhost:3000/any/path/shows/index.html

    // Serve a specific file
    app.StaticFile("/favicon.ico", "./assets/favicon.ico")
    // => http://localhost:3000/favicon.ico

    app.Run(":3000")
}
```

<div align="right">

[&nwarr; Back to top](#readme-top)

</div>

### Middleware & Next

Middleware functions allow you to process requests before they reach their final handler:

```go
package main

import (
    "fmt"

    "github.com/gonoleks/gonoleks"
)

func main() {
    // Initialize a new Gonoleks app
    app := gonoleks.New()

    // Create track group first
    track := app.Group("/track")

    // Add middleware to the app after creating the group
    app.Use(func(c *gonoleks.Context) {
        fmt.Println("🏎️ Warming up the engines...")
        c.Next()
    })

    // Middleware that matches all routes starting with /track
    track.Use(func(c *gonoleks.Context) {
        fmt.Println("🚦 Lights out in 3... 2... 1...")
        c.Next()
    })

    // GET /track/start
    track.GET("/start", func(c *gonoleks.Context) {
        fmt.Println("🏎️💨 GO!")
        c.String(200, "🏁 Full throttle!")
    })

    app.Run(":3000")
}
```

<div align="right">

[&nwarr; Back to top](#readme-top)

</div>

## 🙌🏻 Let's Win Together

If you find Gonoleks useful, please click 👁️ **Watch** button to avoid missing notifications about new versions, and give it a ⭐️ **GitHub Star**.

We’d love your input:

- [Issues][repo_issues_url]: Report bugs or suggest features.
- [Pull Requests][repo_pull_requests_url]: Contribute to the codebase.
- [Discussions][repo_discussions_url]: Share your thoughts and ideas.
- Tweet about the project on your [𝕏 (Twitter)][x_share_url].
- Write about your experience on [Dev.to][dev_to_url], [Medium][medium_url], etc.

Your PRs, issues and any words are welcome! Thanks 🩵

<div align="right">

[&nwarr; Back to top](#readme-top)

</div>

## ☕️ Support the Creator

If you want to support Gonoleks, you can ☕️ [Buy Me a Coffee][buymeacoffee_url].

<div align="right">

[&nwarr; Back to top](#readme-top)

</div>

## 👩🏻‍💻👨🏻‍💻 Contributing

See [Contributing][repo_contributing_url].

<div align="right">

[&nwarr; Back to top](#readme-top)

</div>

## 📄 License
`Gonoleks` is free and open-source software licensed under the [MIT License][repo_license_url].

<!-- Go links -->

[go_url]: https://go.dev
[go_version_img]: https://img.shields.io/badge/Go-1.23+-00ADD8?style=for-the-badge&logo=go
[go_report_img]: https://img.shields.io/badge/Go_report-A+-success?style=for-the-badge&logo=none
[go_report_url]: https://goreportcard.com/report/github.com/gonoleks/gonoleks
[go_dev_url]: https://pkg.go.dev/github.com/gonoleks/gonoleks

<!-- Repository links -->

[repo_url]: https://github.com/gonoleks/gonoleks
[repo_release_img]: https://img.shields.io/github/v/release/gonoleks/gonoleks?style=for-the-badge
[repo_license_img]: https://img.shields.io/github/license/gonoleks/gonoleks?style=for-the-badge
[repo_license_url]: https://github.com/gonoleks/gonoleks/blob/main/LICENSE
[repo_contributing_url]: https://github.com/gonoleks/gonoleks/blob/main/.github/CONTRIBUTING.md
[repo_issues_url]: https://github.com/gonoleks/gonoleks/issues
[repo_pull_requests_url]: https://github.com/gonoleks/gonoleks/pulls
[repo_discussions_url]: https://github.com/orgs/gonoleks/discussions

<!-- README links -->

[examples_url]: https://github.com/gonoleks/examples
[gin_url]: https://github.com/gin-gonic/gin
[fasthttp_url]: https://github.com/valyala/fasthttp
[buymeacoffee_url]: https://buymeacoffee.com/gonoleks
[dev_to_url]: https://dev.to
[medium_url]: https://medium.com
[x_share_url]: https://x.com/intent/post?text=%23Gonoleks%20is%20a%20modern%20Go%20micro-framework%20that%20features%20a%20Gin-inspired%20API%20with%20lightning-faster%20performance%20%E2%9A%A1%EF%B8%8F%20https%3A%2F%2Fgithub.com%2Fgonoleks%2Fgonoleks
