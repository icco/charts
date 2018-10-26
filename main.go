package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"sort"

	"contrib.go.opencensus.io/exporter/stackdriver"
	"contrib.go.opencensus.io/exporter/stackdriver/monitoredresource"
	"github.com/basvanbeek/ocsql"
	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/go-chi/render"
	"github.com/ifo/sanic"
	"github.com/jinzhu/gorm"
	"github.com/wcharczuk/go-chart" //exposes "chart"
	"go.opencensus.io/plugin/ochttp"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/trace"
)

var worker sanic.Worker

// data format
// {
//   format: line,pie,spark
//   data: [
//     { x: 1, y: 2 },
//     { pie data... label + %? }
//     ...
//   ]
//   labels: {
//     x: blah
//     y: blab
//   }
// }
type JSONData struct {
	Format string            `json:"format"`
	Data   []json.RawMessage `json:"data"`
	Labels map[string]string `json:"labels"`
	APIKey string            `json:"api-key"`
}

func (a *JSONData) Bind(r *http.Request) error {
	return nil
}

func main() {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Panicf("DATABASE_URL is empty!")
	}
	driverName, err := ocsql.Register("postgres", ocsql.WithAllTraceOptions())
	if err != nil {
		log.Fatalf("unable to register our ocsql driver: %v\n", err)
	}
	db, err := gorm.Open(driverName, dbURL)

	port := "8080"
	if fromEnv := os.Getenv("PORT"); fromEnv != "" {
		port = fromEnv
	}
	log.Printf("Starting up on http://localhost:%s", port)

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
		trace.ApplyConfig(trace.Config{DefaultSampler: trace.AlwaysSample()})
	}

	isDev := os.Getenv("NAT_ENV") != "production"

	worker := sanic.NewWorker7()

	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.URLFormat)
	r.Use(render.SetContentType(render.ContentTypeJSON))

	r.Post("/chart/new", func(w http.ResponseWriter, r *http.Request) {
		id := worker.NextID()
		idString := worker.IDString(id)
		data := &JSONData{}
		if err := render.Bind(r, data); err != nil {
			log.Printf("Error parsing: %+v", err)
			render.Render(w, r, ErrInvalidRequest(err))
			return
		}

		err = StoreGraphData(r.Context(), data, idString)
		if err != nil {
			log.Printf("Error storing: %+v", err)
			render.Render(w, r, ErrInvalidRequest(err))
			return
		}

		http.Redirect(w, r, fmt.Sprintf("/%s", idString), 302)
	})

	r.Get("/{id:[A-z]+}", func(w http.ResponseWriter, r *http.Request) {
		idString := chi.URLParam(r, "id")
		data := &JSONData{}
		jsonBlob, err := ioutil.ReadFile(fmt.Sprintf("/tmp/%s.json", idString))
		if err != nil {
			log.Printf("Error opening: %+v", err)
			render.Render(w, r, ErrInvalidRequest(err))
			return
		}
		err = json.Unmarshal(jsonBlob, &data)
		if err != nil {
			log.Printf("Error json-ing: %+v", err)
			render.Render(w, r, ErrInvalidRequest(err))
			return
		}

		xs := []float64{}
		ys := []float64{}

		// Sort the X or things look weird
		sort.Slice(data.Data, func(i, j int) bool {
			return data.Data[i].X > data.Data[j].X
		})

		for _, c := range data.Data {
			xs = append(xs, c.X)
			ys = append(ys, c.Y)
		}

		graph := chart.Chart{
			XAxis: chart.XAxis{
				Name:      "X",
				NameStyle: chart.StyleShow(),
				Style:     chart.StyleShow(),
			},
			YAxis: chart.YAxis{
				Name:      "Y",
				NameStyle: chart.StyleShow(),
				Style:     chart.StyleShow(),
			},
			Series: []chart.Series{
				chart.ContinuousSeries{
					XValues: xs,
					YValues: ys,
				},
			},
		}
		log.Printf("Chart: %+v", graph)

		err = graph.Render(chart.PNG, w)
		if err != nil {
			log.Printf("Err rendering: %+v", err)
			render.Render(w, r, ErrInvalidRequest(err))
			return
		}
		w.Header().Set("Content-Type", "image/png")
	})

	r.Get("/healthz", healthCheckHandler)

	h := &ochttp.Handler{
		Handler:          r,
		IsPublicEndpoint: true,
	}
	if err := view.Register(ochttp.DefaultServerViews...); err != nil {
		log.Fatal("Failed to register ochttp.DefaultServerViews")
	}

	log.Fatal(http.ListenAndServe(":"+port, h))
}

func healthCheckHandler(w http.ResponseWriter, r *http.Request) {
	render.JSON(w, r, map[string]string{
		"healthy": "true",
	})
}

type ErrResponse struct {
	Err            error `json:"-"` // low-level runtime error
	HTTPStatusCode int   `json:"-"` // http response status code

	StatusText string `json:"status"`          // user-level status message
	AppCode    int64  `json:"code,omitempty"`  // application-specific error code
	ErrorText  string `json:"error,omitempty"` // application-level error message, for debugging
}

func (e *ErrResponse) Render(w http.ResponseWriter, r *http.Request) error {
	render.Status(r, e.HTTPStatusCode)
	return nil
}

func ErrInvalidRequest(err error) render.Renderer {
	return &ErrResponse{
		Err:            err,
		HTTPStatusCode: 400,
		StatusText:     "Invalid request.",
		ErrorText:      err.Error(),
	}
}

func StoreGraphData(ctx context.Context, d *JSONData, slug string) error {
	return nil
}

func GetGraphData(ctx context.Context, slug string) (*JSONData, error) {
	return nil, nil
}

func RenderGraph(ctx context.Context, slug string, b io.Writer) error {
	return nil
}
