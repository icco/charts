package charts

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log" //TODO: Better log choce
)

type Graph struct {
	ID          string      `json:"id"`
	Description string      `json:"description"`
	Creator     *User       `json:"creator"`
	Data        []DataPoint `json:"data"`
	Type        GraphType   `json:"type"`
}

func GetGraph(ctx context.Context, id string) (*Graph, error) {
	var graph Graph
	var data json.RawMessage
	var userID string
	row := db.QueryRowContext(ctx, "SELECT id, type, description, data, creator_id FROM graphs WHERE id = $1", id)
	err := row.Scan(&graph.ID, &graph.Type, &graph.Description, &data, &userID)
	switch {
	case err == sql.ErrNoRows:
		return nil, fmt.Errorf("No graph with that id.")
	case err != nil:
		return nil, fmt.Errorf("Error running get query: %+v", err)
	}

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
