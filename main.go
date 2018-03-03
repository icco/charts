package main

import (
	"bytes"

	"github.com/go-chi/chi"
	"github.com/wcharczuk/go-chart" //exposes "chart"
)

func main() {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.URLFormat)

	graph := chart.Chart{
		Series: []chart.Series{
			chart.ContinuousSeries{
				XValues: []float64{1.0, 2.0, 3.0, 4.0},
				YValues: []float64{1.0, 2.0, 3.0, 4.0},
			},
		},
	}

	buffer := bytes.NewBuffer([]byte{})
	err := graph.Render(chart.PNG, buffer)
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
