package calcium

import (
	"context"

	"github.com/projecteru2/core/types"
)

// AddPod add pod
func (c *Calcium) AddPod(ctx context.Context, podname, desc string) (*types.Pod, error) {
	return c.store.AddPod(ctx, podname, desc)
}

// RemovePod remove pod
func (c *Calcium) RemovePod(ctx context.Context, podname string) error {
	return c.withNodesLocked(ctx, podname, "", nil, true, func(nodes map[string]*types.Node) error {
		// TODO dissociate container to node
		// TODO remove node first
		return c.store.RemovePod(ctx, podname)
	})
}

// GetPod get one pod
func (c *Calcium) GetPod(ctx context.Context, podname string) (*types.Pod, error) {
	return c.store.GetPod(ctx, podname)
}

// ListPods show pods
func (c *Calcium) ListPods(ctx context.Context) ([]*types.Pod, error) {
	return c.store.GetAllPods(ctx)
}
