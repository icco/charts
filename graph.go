package charts

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log" // TODO: Better log choce
	"time"

	"github.com/google/uuid"
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

func GetGraph(ctx context.Context, id string) (*Graph, error) {
	var graph Graph
	var data json.RawMessage
	var userID string
	var graphType string

	row := db.QueryRowContext(ctx, "SELECT id, type, description, data, creator_id FROM graphs WHERE id = $1", id)
	err := row.Scan(&graph.ID, &graphType, &graph.Description, &data, &userID)

	switch {
	case err == sql.ErrNoRows:
		return nil, fmt.Errorf("No graph with that id.")
	case err != nil:
		return nil, fmt.Errorf("Error running get query: %+v", err)
	}

	graph.Type = GraphType(graphType)

	user, err := GetUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	graph.Creator = user

	err = graph.parseJSONToData(data)
	if err != nil {
		return nil, err
	}

	return &graph, nil
}

func (g *Graph) parseJSONToData(data json.RawMessage) error {
	var rawData []json.RawMessage
	g.Data = []DataPoint{}

	err := json.Unmarshal(data, &rawData)
	if err != nil {
		log.Printf("Problem parsing json: %+v", err)
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
	return Graph{}, fmt.Errorf("Not implemented yet.")
}

func CreateTimeseriesGraph(ctx context.Context, input NewTimeseriesGraph) (Graph, error) {
	return Graph{}, fmt.Errorf("Not implemented yet.")
}
