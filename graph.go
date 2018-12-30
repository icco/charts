package charts

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/wcharczuk/go-chart" //exposes "chart"
)

type Graph struct {
	ID          string      `json:"id"`
	Description string      `json:"description"`
	Creator     *User       `json:"creator"`
	Data        []DataPoint `json:"data"`
	Type        GraphType   `json:"type"`
	Created     time.Time   `json:"created"`
	Modified    time.Time   `json:"modified"`
}

// URL returns a url someone could view this page at.
func (g *Graph) URL(ctx context.Context) string {
	// TODO: Make this dependent on where we are running.
	host := "https://chartopia.app"
	return fmt.Sprintf("%s/graph/%s", host, g.ID)
}

func GetGraph(ctx context.Context, id string) (*Graph, error) {
	var graph Graph
	var data json.RawMessage
	var graphType string

	row := db.QueryRowContext(ctx, "SELECT id, type, description, data FROM graphs WHERE id = $1", id)
	err := row.Scan(&graph.ID, &graphType, &graph.Description, &data)

	switch {
	case err == sql.ErrNoRows:
		return nil, fmt.Errorf("no graph with that id")
	case err != nil:
		return nil, fmt.Errorf("error running get query: %+v", err)
	}

	graph.Type = GraphType(graphType)

	// TODO: Add users back in.
	//user, err := GetUser(ctx, userID)
	//if err != nil {
	//	return nil, err
	//}
	//graph.Creator = user

	err = graph.parseJSONToData(data)
	if err != nil {
		return nil, err
	}

	return &graph, nil
}

// GraphCount returns the number of entries in the graph table. If there are
// any errors, it logs them and returns 0.
func GraphCount(ctx context.Context) int64 {
	var cnt int64
	err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM graphs").Scan(&cnt)
	if err != nil {
		log.WithError(err).Warn("Error getting count")
		return 0
	}

	return cnt
}

func (g *Graph) parseJSONToData(data json.RawMessage) error {
	var rawData []json.RawMessage
	g.Data = []DataPoint{}

	err := json.Unmarshal(data, &rawData)
	if err != nil {
		log.WithError(err).Error("problem parsing json")
		return err
	}

	ret := make([]DataPoint, len(rawData))
	for i, r := range rawData {
		switch g.Type {
		case GraphTypeLine:
			var pair PairPoint
			err := json.Unmarshal(r, &pair)
			if err != nil {
				return err
			}
			ret[i] = pair
		case GraphTypePie:
			var pie PiePoint
			err := json.Unmarshal(r, &pie)
			if err != nil {
				return err
			}
			ret[i] = pie
		case GraphTypeTimeseries:
			var pair TimePoint
			err := json.Unmarshal(r, &pair)
			if err != nil {
				return err
			}
			ret[i] = pair
		}
	}

	g.Data = ret
	return nil
}

func (g *Graph) Save(ctx context.Context) error {
	if g.ID == "" {
		uid, err := uuid.NewRandom()
		if err != nil {
			return err
		}
		g.ID = uid.String()
	}

	j, err := json.Marshal(g.Data)
	if err != nil {
		return err
	}

	if _, err := db.ExecContext(
		ctx,
		`
INSERT INTO graphs(id, description, type, data, created_at, modified_at)
VALUES ($1, $2, $3, $4, $5, $5)
ON CONFLICT (id) DO UPDATE
SET (description, type, data, modified_at) = ($2, $3, $4, $5)
WHERE graphs.id = $1;
`,
		g.ID,
		g.Description,
		g.Type,
		j,
		time.Now()); err != nil {
		return err
	}

	return nil
}

// Render writes a PNG of the graph to an io.Writer.
func (g *Graph) Render(ctx context.Context, w io.Writer) error {
	renderer := chart.PNG
	if len(g.Data) > 0 {
		switch g.Type {
		case GraphTypeLine:
			d := make([]PairPoint, len(g.Data))
			for i, r := range g.Data {
				d[i] = r.(PairPoint)
			}
			return generateLineGraph(d, renderer, w)
		case GraphTypePie:
			d := make([]PiePoint, len(g.Data))
			for i, r := range g.Data {
				d[i] = r.(PiePoint)
			}
			return generatePieGraph(d, renderer, w)
		case GraphTypeTimeseries:
			d := make([]TimePoint, len(g.Data))
			for i, r := range g.Data {
				d[i] = r.(TimePoint)
			}
			return generateTimeGraph(d, renderer, w)
		}
	}

	return fmt.Errorf("don't know how to render this graph type")
}

func generateLineGraph(data []PairPoint, renderer chart.RendererProvider, w io.Writer) error {
	// Sort the X or things look weird
	sort.Slice(data, func(i, j int) bool {
		return data[i].X > data[j].X
	})

	xs := make([]float64, len(data))
	ys := make([]float64, len(data))

	for i, c := range data {
		xs[i] = c.X
		ys[i] = c.Y
	}

	return chart.Chart{
		XAxis: chart.XAxis{
			Style: chart.StyleShow(),
		},
		YAxis: chart.YAxis{
			Style: chart.StyleShow(),
		},
		Series: []chart.Series{
			chart.ContinuousSeries{
				XValues: xs,
				YValues: ys,
			},
		},
	}.Render(renderer, w)
}

func generateTimeGraph(data []TimePoint, renderer chart.RendererProvider, w io.Writer) error {
	// Sort the X or things look weird
	sort.Slice(data, func(i, j int) bool {
		return data[i].Timestamp.After(data[j].Timestamp)
	})

	xs := make([]time.Time, len(data))
	ys := make([]float64, len(data))

	for i, c := range data {
		xs[i] = c.Timestamp
		ys[i] = c.Value
	}

	return chart.Chart{
		XAxis: chart.XAxis{
			Style: chart.StyleShow(),
		},
		YAxis: chart.YAxis{
			Style: chart.StyleShow(),
		},
		Series: []chart.Series{
			chart.TimeSeries{
				XValues: xs,
				YValues: ys,
			},
		},
	}.Render(renderer, w)
}

func generatePieGraph(data []PiePoint, renderer chart.RendererProvider, w io.Writer) error {
	vals := []chart.Value{}
	for _, r := range data {
		vals = append(vals, chart.Value{Value: r.Percent})
	}

	return chart.PieChart{Values: vals}.Render(renderer, w)
}

func CreateLineGraph(ctx context.Context, input NewLineGraph) (Graph, error) {
	g := Graph{}
	g.Type = GraphTypeLine

	if input.Description != nil {
		g.Description = *input.Description
	}

	g.Data = make([]DataPoint, len(input.Data))
	for i, r := range input.Data {
		p := PairPoint{X: r.X, Y: r.Y}
		p.Meta = make([]*Meta, len(r.Meta))
		for i, m := range r.Meta {
			p.Meta[i] = &Meta{Key: m.Key, Value: m.Value}
		}
		g.Data[i] = p
	}

	err := g.Save(ctx)
	return g, err
}

func CreatePieGraph(ctx context.Context, input NewPieGraph) (Graph, error) {
	g := Graph{}
	g.Type = GraphTypePie

	if input.Description != nil {
		g.Description = *input.Description
	}

	g.Data = make([]DataPoint, len(input.Data))
	for i, r := range input.Data {
		p := PiePoint{Percent: r.Percent}
		p.Meta = make([]*Meta, len(r.Meta))
		for i, m := range r.Meta {
			p.Meta[i] = &Meta{Key: m.Key, Value: m.Value}
		}
		g.Data[i] = p
	}

	err := g.Save(ctx)
	return g, err
}

func CreateTimeseriesGraph(ctx context.Context, input NewTimeseriesGraph) (Graph, error) {
	g := Graph{}
	g.Type = GraphTypeTimeseries

	if input.Description != nil {
		g.Description = *input.Description
	}

	g.Data = make([]DataPoint, len(input.Data))
	for i, r := range input.Data {
		p := TimePoint{Timestamp: r.Timestamp, Value: r.Value}
		p.Meta = make([]*Meta, len(r.Meta))
		for i, m := range r.Meta {
			p.Meta[i] = &Meta{Key: m.Key, Value: m.Value}
		}
		g.Data[i] = p
	}

	err := g.Save(ctx)
	return g, err
}
