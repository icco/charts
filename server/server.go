package main

import (
	"context"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"contrib.go.opencensus.io/exporter/stackdriver"
	"contrib.go.opencensus.io/exporter/stackdriver/monitoredresource"
	"contrib.go.opencensus.io/exporter/stackdriver/propagation"
	gql "github.com/99designs/gqlgen/graphql"
	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/apollotracing"
	"github.com/99designs/gqlgen/graphql/handler/extension"
	"github.com/99designs/gqlgen/graphql/handler/lru"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/icco/charts"
	"github.com/icco/gutil/logging"
	"github.com/vektah/gqlparser/v2/gqlerror"
	"go.opencensus.io/plugin/ochttp"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/trace"
	"go.uber.org/zap"
	"gopkg.in/unrolled/render.v1"
	"gopkg.in/unrolled/secure.v1"
)

var (
	// Renderer is a renderer for all occasions. These are our preferred default options.
	// See:
	//  - https://github.com/unrolled/render/blob/v1/README.md
	//  - https://godoc.org/gopkg.in/unrolled/render.v1
	Renderer = render.New(render.Options{
		Charset:                   "UTF-8",
		Directory:                 "./server/views",
		DisableHTTPErrorRendering: false,
		Extensions:                []string{".tmpl", ".html"},
		IndentJSON:                false,
		IndentXML:                 true,
		Layout:                    "layout",
		RequirePartials:           true,
		Funcs:                     []template.FuncMap{{}},
	})

	dbURL = os.Getenv("DATABASE_URL")
	log   = logging.Must(logging.NewLogger(charts.Service))
)

func main() {
	if dbURL == "" {
		log.Fatalw("DATABASE_URL is empty!")
	}

	if _, err := charts.InitDB(dbURL); err != nil {
		log.Fatalw("Init DB", zap.Error(err))
	}

	port := "8080"
	if fromEnv := os.Getenv("PORT"); fromEnv != "" {
		port = fromEnv
	}
	log.Infow("Starting up", "host", fmt.Sprintf("http://localhost:%s", port))

	if os.Getenv("ENABLE_STACKDRIVER") != "" {
		labels := &stackdriver.Labels{}
		labels.Set("app", charts.Service, "The name of the current app.")
		sd, err := stackdriver.NewExporter(stackdriver.Options{
			ProjectID:               "icco-cloud",
			MonitoredResource:       monitoredresource.Autodetect(),
			DefaultMonitoringLabels: labels,
			DefaultTraceAttributes:  map[string]interface{}{"/http/host": "chartopia.app"},
		})

		if err != nil {
			log.Fatalw("Failed to create the Stackdriver exporter", zap.Error(err))
		}
		defer sd.Flush()

		view.RegisterExporter(sd)
		trace.RegisterExporter(sd)
		trace.ApplyConfig(trace.Config{
			DefaultSampler: trace.AlwaysSample(),
		})
	}

	gh := handler.New(charts.NewExecutableSchema(charts.New()))
	gh.AddTransport(transport.Websocket{KeepAlivePingInterval: 10 * time.Second})
	gh.AddTransport(transport.Options{})
	gh.AddTransport(transport.GET{})
	gh.AddTransport(transport.POST{})
	gh.AddTransport(transport.MultipartForm{})
	gh.Use(apollotracing.Tracer{})
	gh.SetQueryCache(lru.New(1000))
	gh.Use(extension.Introspection{})

	gh.SetErrorPresenter(func(ctx context.Context, e error) *gqlerror.Error {
		err := gql.DefaultErrorPresenter(ctx, e)

		log.Warnw("graphql request error", zap.Error(e))
		if strings.Contains(e.Error(), "forbidden") {
			return gqlerror.Errorf("forbidden: not a valid user")
		}

		return err
	})
	gh.SetRecoverFunc(func(ctx context.Context, err interface{}) error {
		if e, ok := err.(error); !ok {
			log.Errorw("graphql fatal request error", "error", err)
		} else {
			log.Errorw("graphql fatal request error", zap.Error(e))
		}

		return fmt.Errorf("Internal server error!")
	})

	gh.AroundResponses(GqlLoggingMiddleware)

	isDev := os.Getenv("NAT_ENV") != "production"

	r := chi.NewRouter()
	r.Use(middleware.RealIP)
	r.Use(logging.Middleware(log.Desugar(), "icco-cloud"))
	r.Use(cors.New(cors.Options{
		AllowCredentials:   true,
		OptionsPassthrough: true,
		AllowedOrigins:     []string{"*"},
		AllowedMethods:     []string{"GET", "POST", "OPTIONS"},
		AllowedHeaders:     []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:     []string{"Link"},
		MaxAge:             300, // Maximum value not ignored by any of major browsers
	}).Handler)

	r.NotFound(notFoundHandler)

	// Stuff that does not ssl redirect
	r.Group(func(r chi.Router) {
		r.Use(secure.New(secure.Options{
			BrowserXssFilter:   true,
			ContentTypeNosniff: true,
			FrameDeny:          true,
			HostsProxyHeaders:  []string{"X-Forwarded-Host"},
			IsDevelopment:      isDev,
			SSLProxyHeaders:    map[string]string{"X-Forwarded-Proto": "https"},
		}).Handler)

		r.Get("/healthz", healthCheckHandler)
	})

	// Everything that does SSL only
	r.Group(func(r chi.Router) {
		r.Use(secure.New(secure.Options{
			BrowserXssFilter:     true,
			ContentTypeNosniff:   true,
			FrameDeny:            true,
			HostsProxyHeaders:    []string{"X-Forwarded-Host"},
			IsDevelopment:        isDev,
			SSLProxyHeaders:      map[string]string{"X-Forwarded-Proto": "https"},
			SSLRedirect:          !isDev,
			STSIncludeSubdomains: true,
			STSPreload:           true,
			STSSeconds:           315360000,
		}).Handler)

		r.Get("/", func(w http.ResponseWriter, r *http.Request) {
			Renderer.HTML(w, http.StatusOK, "index", map[string]string{
				"count": strconv.FormatInt(charts.GraphCount(r.Context()), 10),
			})
		})

		r.Get("/static/{filename}", func(w http.ResponseWriter, r *http.Request) {
			filename := chi.URLParam(r, "filename")
			base := "/go/src/github.com/icco/charts"
			if isDev {
				base = "."
			}
			http.ServeFile(w, r, fmt.Sprintf("%s/server/views/static/%s", base, filename))
		})

		r.Handle("/play", playground.Handler("charts", "/graphql"))
		r.Handle("/graphql", gh)
		r.Get("/graph/{graphID}", renderGraphHandler)
	})
	h := &ochttp.Handler{
		Handler:     r,
		Propagation: &propagation.HTTPFormat{},
	}
	if err := view.Register([]*view.View{
		ochttp.ServerRequestCountView,
		ochttp.ServerResponseCountByStatusCode,
	}...); err != nil {
		log.Fatalw("Failed to register ochttp.DefaultServerViews")
	}

	log.Fatal(http.ListenAndServe(":"+port, h))
}

func renderGraphHandler(w http.ResponseWriter, r *http.Request) {
	// get graph
	gid := chi.URLParam(r, "graphID")
	g, err := charts.GetGraph(r.Context(), gid)
	if err != nil {
		log.Errorw("get graph", zap.Error(err))
		http.Error(w, "Error getting graph.", http.StatusNotFound)
		return
	}

	// render graph
	w.Header().Set("Content-Type", "image/png")
	err = g.Render(r.Context(), w)
	if err != nil {
		log.Errorw("render graph", zap.Error(err))
		http.Error(w, "Error rendering graph.", http.StatusInternalServerError)
		return
	}
}

func healthCheckHandler(w http.ResponseWriter, r *http.Request) {
	Renderer.JSON(w, http.StatusOK, map[string]string{
		"healthy": "true",
	})
}

func notFoundHandler(w http.ResponseWriter, r *http.Request) {
	Renderer.JSON(w, http.StatusNotFound, map[string]string{
		"error": "404: This page could not be found",
	})
}

// GqlLoggingMiddleware is a middleware for gqlgen that logs all gql requests to debug.
func GqlLoggingMiddleware(ctx context.Context, next gql.ResponseHandler) *gql.Response {
	rctx := gql.GetOperationContext(ctx)

	// We do this because RequestContext has fields that can't be easily
	// serialized in json, and we don't care about them.
	subsetContext := map[string]interface{}{
		"query":     rctx.RawQuery,
		"variables": rctx.Variables,
		"name":      rctx.OperationName,
		"stats":     rctx.Stats,
	}

	log.Debugw("request gql", "gql", subsetContext)

	return next(ctx)
}
