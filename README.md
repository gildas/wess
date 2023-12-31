# wess

![GoVersion](https://img.shields.io/github/go-mod/go-version/gildas/wess)
[![GoDoc](https://img.shields.io/badge/go.dev-reference-007d9c?logo=go&logoColor=white&style=flat-square)](https://pkg.go.dev/github.com/gildas/wess)
[![License](https://img.shields.io/github/license/gildas/wess)](https://github.com/gildas/wess/blob/master/LICENSE)
[![Report](https://goreportcard.com/badge/github.com/gildas/wess)](https://goreportcard.com/report/github.com/gildas/wess)  

![master](https://img.shields.io/badge/branch-master-informational)
[![Test](https://github.com/gildas/wess/actions/workflows/test.yml/badge.svg?branch=master)](https://github.com/gildas/wess/actions/workflows/test.yml)
[![codecov](https://codecov.io/gh/gildas/wess/branch/master/graph/badge.svg?token=gFCzS9b7Mu)](https://codecov.io/gh/gildas/wess/branch/master)

![dev](https://img.shields.io/badge/branch-dev-informational)
[![Test](https://github.com/gildas/wess/actions/workflows/test.yml/badge.svg?branch=dev)](https://github.com/gildas/wess/actions/workflows/test.yml)
[![codecov](https://codecov.io/gh/gildas/wess/branch/dev/graph/badge.svg?token=gFCzS9b7Mu)](https://codecov.io/gh/gildas/wess/branch/dev)

WEb Simple Server

## Running a WESS instance

### Basics

Running a WESS instance is very easy, by default on port 80 all interfaces:

```go
func main() {
  server := wess.NewServer(wess.ServerOptions{})
  shutdown, stop, _ := server.Start(context.Background())
  err = <-shutdown
}
```

`err` will contain the eventual errors when the server shuts down and `stop` is a `chan` that allows you to stop the server programmatically.

Of course that server does not serve much...

You can change the port, give an address to listen to:

```go
server := wess.NewServer(wess.ServerOptions{
  Address: "192.168.1.1",
  Port:    8000,
})
```

You can also overwrite the default handlers used when a route is not found or a method is not Allowed:

```go
server := wess.NewServer(wess.ServerOptions{
  Logger:                  log,
  NotFoundHandler:         notFoundHandler(log),
  MethodNotAllowedHandler: methodNotAllowedHandler(Options),
})
```

If you add a `ProbePort`, `wess` will also serve some _health_ routes for Kubernetes or other probe oriented environments. These following routes are available:

- `/healthz/liveness`
- `/healthz/readiness`

You can change the root path from `/healthz` with the `HealthRootPath` option:

```go
server := wess.NewServer(wess.ServerOptions{
  ProbePort:      32000,
  HealthRootPath: "/probez",
})
```

**Note:** If the probe port is the same as the main port, all routes are handled by the same web server. Otherwise, 2 web servers are instantiated.

### Adding routes

You can add a simple route with `AddRoute` and `AddRouteWithFunc`:

```go
server.AddRoute("GET", "/something", somethingHandler)
server.AddRouteWithFunc("GET", "/somewhere", somewhereFunc)
```

The first method accepts a [http.Handler](https://pkg.go.dev/net/http#Handler) while the second accepts a [http.HandlerFunc](https://pkg.go.dev/net/http#HandlerFunc).

Here is an embedded example:

```go
server.AddRouteWithFunc("GET", "/hello", func(w http.ResponseWriter, r *http.Request) {
  log := logger.Must(logger.FromContext(r.Context())).Child(nil, "hello")

  w.WriteHeader(http.StatusOK)
  w.Header().Add("Content-Type", "text/plain")
  written, _ := w.Write([]byte("Hello, World!"))
  log.Debugf("Witten %d bytes", written)
})
```

For more complex cases, you can ask for a [SubRouter](https://pkg.go.dev/github.com/gorilla/mux#Router):

```go
main() {
  // ...
  APIRoutes(server.SubRouter("/api/v1"), dbClient)
}

func APIRoutes(router *mux.Router, db *db.Client) {
  router.Method("GET").Path("/users").Handler(GetUsers(db))
}
```

(See the [vue-with-api](samples/vue-with-api/README.md) sample for a complete implementation)

### Adding a frontend

To add a frontend, the easiest is to use [vite](https://vitejs.dev). You can also use [webpack](https://webpack.js.org). As long as you can bundle all the distribution files in the same folder.

In the folder you want to write your `wess` application, create the frontend instance (Here, using [Vue](https://vuejs.org). You can also use [react](https://react.dev)):

```sh
npm create vite@latest frontent -- --template vue
```

or with [yarn](https://yarnpkg.com/):

```sh
yarn create vite frontend --template vue
```

During the development phase of the frontend, you should run the dev server directly from the frontend folder:

```sh
cd frontend
npm install
npm run dev
```

or with [yarn](https://yarnpkg.com/):

```sh
cd frontend
yarn install
yarn dev
```

Once the frontend is done, build the project:

```sh
npm run build
```

or with [yarn](https://yarnpkg.com/):

```sh
yarn build
```

And add a `wess` server in the parent folder of the frontend:

```go
var (
  //go:embed all:frontend/dist
  frontendFS embed.FS
)

func main() {
  server := wess.NewServer(wess.ServerOptions{
    Port: 8080,
  })
  _ = server.AddFrontend("/", frontendFS, "frontend/dist")
  shutdown, stop, _ := server.Start(context.Background())
  <-shutdown
}
```

Then, create a single binary that will contain the server and the frontend code:

```sh
go build .
```

That's it, you now have a single small executable file!

## Running the samples

The `samples` folder contains a few examples of `wess` in action.

- [samples/vue](samples/vue/README.md) contains a simple [Vue](https://vuejs.org) project
- [samples/vue-with-api](samples/vue-with-api/README.md) contains a simple [Vue](https://vuejs.org) project that calls an API ran by `wess`
