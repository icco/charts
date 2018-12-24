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

type mutationResolver struct{ *Resolver }

func (r *mutationResolver) CreateLineGraph(ctx context.Context, input NewLineGraph) (Graph, error) {
	panic("not implemented")
}
func (r *mutationResolver) CreatePieGraph(ctx context.Context, input NewPieGraph) (Graph, error) {
	panic("not implemented")
}
func (r *mutationResolver) CreateTimeseriesGraph(ctx context.Context, input NewTimeseriesGraph) (Graph, error) {
	panic("not implemented")
}

type queryResolver struct{ *Resolver }

func (r *queryResolver) GetGraph(ctx context.Context, id string) (*Graph, error) {
	panic("not implemented")
}
