package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"sort"

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
	port := "8080"
	if fromEnv := os.Getenv("PORT"); fromEnv != "" {
		port = fromEnv
	}
	log.Printf("Starting up on %s", port)

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
		data := &JsonData{}
		if err := render.Bind(r, data); err != nil {
			log.Printf("Error parsing: %+v", err)
			render.Render(w, r, ErrInvalidRequest(err))
			return
		}

		log.Printf("recieved: %+v", data)

		b, err := json.Marshal(data)
		if err != nil {
			log.Printf("Error json-ing: %+v", err)
			render.Render(w, r, ErrInvalidRequest(err))
			return
		}

		err = ioutil.WriteFile(fmt.Sprintf("/tmp/%s.json", idString), b, 0644)
		if err != nil {
			log.Printf("Error saving: %+v", err)
			render.Render(w, r, ErrInvalidRequest(err))
			return
		}

		http.Redirect(w, r, fmt.Sprintf("/%s", idString), 302)
	})

	r.Get("/{id:[A-z]+}", func(w http.ResponseWriter, r *http.Request) {
		idString := chi.URLParam(r, "id")
		data := &JsonData{}
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

	log.Printf("Server listening on port %s", port)
	log.Fatal(http.ListenAndServe(":"+port, r))
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
