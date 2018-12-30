package main

import (
	"context"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"strconv"

	"contrib.go.opencensus.io/exporter/stackdriver"
	"contrib.go.opencensus.io/exporter/stackdriver/monitoredresource"
	"contrib.go.opencensus.io/exporter/stackdriver/propagation"
	"github.com/99designs/gqlgen-contrib/gqlopencensus"
	"github.com/99designs/gqlgen/graphql"
	"github.com/99designs/gqlgen/handler"
	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/go-chi/cors"
	"github.com/icco/charts"
	"go.opencensus.io/plugin/ochttp"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/trace"
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

	log = charts.InitLogging()
)

func main() {
	if dbURL == "" {
		log.Fatalf("DATABASE_URL is empty!")
	}

	_, err := charts.InitDB(dbURL)
	if err != nil {
		log.Fatalf("Init DB: %+v", err)
	}

	port := "8080"
	if fromEnv := os.Getenv("PORT"); fromEnv != "" {
		port = fromEnv
	}
	log.Debugf("Starting up on http://localhost:%s", port)

	if os.Getenv("ENABLE_STACKDRIVER") != "" {
		sd, err := stackdriver.NewExporter(stackdriver.Options{
			ProjectID:               "icco-cloud",
			MetricPrefix:            "charts",
			MonitoredResource:       monitoredresource.Autodetect(),
			DefaultMonitoringLabels: &stackdriver.Labels{},
		})

		if err != nil {
			log.Fatalf("Failed to create the Stackdriver exporter: %v", err)
		}
		defer sd.Flush()

		view.RegisterExporter(sd)
		trace.RegisterExporter(sd)
		trace.ApplyConfig(trace.Config{
			DefaultSampler: trace.AlwaysSample(),
		})
	}

	isDev := os.Getenv("NAT_ENV") != "production"

	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(charts.LoggingMiddleware())

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
			http.ServeFile(w, r, fmt.Sprintf("./server/views/static/%s", filename))
		})

		r.Handle("/play", handler.Playground("graphql", "/graphql"))
		r.Handle("/graphql", buildGraphQLHandler())
		r.Get("/graph/{graphID}", renderGraphHandler)
	})
	h := &ochttp.Handler{
		Handler:     r,
		Propagation: &propagation.HTTPFormat{},
	}
	if err := view.Register(ochttp.DefaultServerViews...); err != nil {
		log.Fatal("Failed to register ochttp.DefaultServerViews")
	}

	log.Fatal(http.ListenAndServe(":"+port, h))
}

func buildGraphQLHandler() http.HandlerFunc {
	return handler.GraphQL(
		charts.NewExecutableSchema(charts.New()),
		handler.RecoverFunc(func(ctx context.Context, intErr interface{}) error {
			err, ok := intErr.(error)
			if ok {
				log.WithError(err).Error("Error seen during graphql")
			}
			return errors.New("Fatal message seen when processing request")
		}),
		handler.CacheSize(512),
		handler.RequestMiddleware(func(ctx context.Context, next func(ctx context.Context) []byte) []byte {
			rctx := graphql.GetRequestContext(ctx)

			// We do this because RequestContext has fields that can't be easily
			// serialized in json, and we don't care about them.
			subsetContext := map[string]interface{}{
				"query":      rctx.RawQuery,
				"variables":  rctx.Variables,
				"extensions": rctx.Extensions,
			}

			log.WithField("gql", subsetContext).Debugf("request gql")

			return next(ctx)
		}),
		handler.Tracer(gqlopencensus.New()),
	)
}

func renderGraphHandler(w http.ResponseWriter, r *http.Request) {
	// get graph
	gid := chi.URLParam(r, "graphID")
	g, err := charts.GetGraph(r.Context(), gid)
	if err != nil {
		log.WithError(err).Error("get graph")
		http.Error(w, "Error getting graph.", http.StatusNotFound)
		return
	}

	// render graph
	w.Header().Set("Content-Type", "image/png")
	err = g.Render(r.Context(), w)
	if err != nil {
		log.WithError(err).Error("render graph")
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
