package main

import (
	"log"
	"net/http"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/ifo/sanic"
	"github.com/wcharczuk/go-chart" //exposes "chart"
)

var worker sanic.Worker

func main() {
	worker := sanic.NewWorker7()

	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.URLFormat)

	r.Get("/chart/new", func(w http.ResponseWriter, r *http.Request) {
		graph := chart.Chart{
			Series: []chart.Series{
				chart.ContinuousSeries{
					XValues: []float64{1.0, 2.0, 3.0, 4.0},
					YValues: []float64{1.0, 2.0, 3.0, 4.0},
				},
			},
		}

		id := worker.NextID()
		idString := worker.IDString(id)
		log.Printf(idString)

		w.Header().Set("Content-Type", "image/png")
		err := graph.Render(chart.PNG, w)
		if err != nil {
			log.Printf("Err rendering: %+v", err)
		}
	})

	http.ListenAndServe(":8080", r)
}

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
