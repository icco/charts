package charts

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log" //TODO: Better log choce
)

type Graph struct {
	ID          string       `json:"id"`
	Description string       `json:"description"`
	Creator     *User        `json:"creator"`
	Data        []*DataPoint `json:"data"`
}

func GetGraph(ctx context.Context, id string) (*Graph, error) {
	var graph Graph
	var data json.RawMessage
	var userID string
	row := db.QueryRowContext(ctx, "SELECT id, description, data, creator_id FROM graphs WHERE id = $1", id)
	err := row.Scan(&graph.ID, &graph.Description, &data, &userID)
	switch {
	case err == sql.ErrNoRows:
		return nil, fmt.Errorf("No graph with that id.")
	case err != nil:
		return nil, fmt.Errorf("Error running get query: %+v", err)
	}

	err, user := GetUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	graph.Creator = user

	graph.Data = parseJSON(data)

	return &graph, nil
}

func parseJSON(data json.RawMessage) []*DataPoint {
	var rawData []interface{}

	err := json.Unmarshal(data, &rawData)
	if err != nil {
		log.Printf("Problem parsing json: %+v", err)
		return []*DataPoint{}
	}

	ret := make([]*DataPoint, len(rawData))
	for i, r := range rawData {
	}

	return ret
}
