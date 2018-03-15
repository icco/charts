package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/go-chi/render"
	"github.com/ifo/sanic"
	"github.com/wcharczuk/go-chart" //exposes "chart"
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
type JsonData struct {
	Format string            `json:"format"`
	Data   []Coordinate      `json:"data"`
	Labels map[string]string `json:"labels"`
}

type Coordinate struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

func (a *JsonData) Bind(r *http.Request) error {
	return nil
}

func main() {
	worker := sanic.NewWorker7()

	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.URLFormat)
	r.Use(render.SetContentType(render.ContentTypeJSON))

	r.Post("/chart/new", func(w http.ResponseWriter, r *http.Request) {
		data := &JsonData{}
		if err := render.Bind(r, data); err != nil {
			render.Render(w, r, ErrInvalidRequest(err))
			return
		}

		log.Printf("recieved: %+v", data)

		id := worker.NextID()
		idString := worker.IDString(id)
		http.Redirect(w, r, fmt.Sprintf("/%s", idString), 302)
	})

	r.Get("/:id", func(w http.ResponseWriter, r *http.Request) {
		graph := chart.Chart{
			Series: []chart.Series{
				chart.ContinuousSeries{
					XValues: []float64{1.0, 2.0, 3.0, 4.0},
					YValues: []float64{1.0, 2.0, 3.0, 4.0},
				},
			},
		}
		w.Header().Set("Content-Type", "image/png")
		err := graph.Render(chart.PNG, w)
		if err != nil {
			log.Printf("Err rendering: %+v", err)
		}
	})

	http.ListenAndServe(":8080", r)
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
