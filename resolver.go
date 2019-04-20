package charts

import (
	"context"
)

type Resolver struct{}

func (r *Resolver) Mutation() MutationResolver {
	return &mutationResolver{r}
}
func (r *Resolver) Query() QueryResolver {
	return &queryResolver{r}
}

// New returns a Config that has all of the proper settings for this graphql
// server.
func New() Config {
	c := Config{
		Resolvers: &Resolver{},
	}

	return c
}

type mutationResolver struct{ *Resolver }

func (r *mutationResolver) CreateLineGraph(ctx context.Context, input NewLineGraph) (*Graph, error) {
	return CreateLineGraph(ctx, input)
}

func (r *mutationResolver) CreatePieGraph(ctx context.Context, input NewPieGraph) (*Graph, error) {
	return CreatePieGraph(ctx, input)
}

func (r *mutationResolver) CreateTimeseriesGraph(ctx context.Context, input NewTimeseriesGraph) (*Graph, error) {
	return CreateTimeseriesGraph(ctx, input)
}

type queryResolver struct{ *Resolver }

func (r *queryResolver) GetGraph(ctx context.Context, id string) (*Graph, error) {
	return GetGraph(ctx, id)
}
