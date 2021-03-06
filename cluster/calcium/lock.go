package calcium

import (
	"context"
	"fmt"
	"time"

	"github.com/projecteru2/core/cluster"
	"github.com/projecteru2/core/lock"
	"github.com/projecteru2/core/types"
	log "github.com/sirupsen/logrus"
)

func (c *Calcium) doLock(ctx context.Context, name string, timeout time.Duration) (lock.DistributedLock, error) {
	lock, err := c.store.CreateLock(name, timeout)
	if err != nil {
		return nil, err
	}
	if err = lock.Lock(ctx); err != nil {
		return nil, err
	}
	return lock, nil
}

func (c *Calcium) doUnlock(lock lock.DistributedLock, msg string) error {
	log.Debugf("[doUnlock] Unlock %s", msg)
	return lock.Unlock(context.Background())
}

func (c *Calcium) doUnlockAll(locks map[string]lock.DistributedLock) {
	for n, lock := range locks {
		// force unlock
		if err := c.doUnlock(lock, n); err != nil {
			log.Errorf("[doUnlockAll] Unlock failed %v", err)
			continue
		}
	}
}

func (c *Calcium) withContainerLocked(ctx context.Context, ID string, f func(container *types.Container) error) error {
	return c.withContainersLocked(ctx, []string{ID}, func(containers map[string]*types.Container) error {
		return f(containers[ID])
	})
}

func (c *Calcium) withNodeLocked(ctx context.Context, nodename string, f func(node *types.Node) error) error {
	return c.withNodesLocked(ctx, "", nodename, nil, true, func(nodes map[string]*types.Node) error {
		return f(nodes[nodename])
	})
}

func (c *Calcium) withContainersLocked(ctx context.Context, IDs []string, f func(containers map[string]*types.Container) error) error {
	containers := map[string]*types.Container{}
	locks := map[string]lock.DistributedLock{}
	defer func() { c.doUnlockAll(locks) }()
	cs, err := c.GetContainers(ctx, IDs)
	if err != nil {
		return err
	}
	for _, container := range cs {
		lock, err := c.doLock(ctx, fmt.Sprintf(cluster.ContainerLock, container.ID), c.config.LockTimeout)
		if err != nil {
			return err
		}
		locks[container.ID] = lock
		containers[container.ID] = container
	}
	return f(containers)
}

func (c *Calcium) withNodesLocked(ctx context.Context, podname, nodename string, labels map[string]string, all bool, f func(nodes map[string]*types.Node) error) error {
	nodes := map[string]*types.Node{}
	locks := map[string]lock.DistributedLock{}
	defer func() { c.doUnlockAll(locks) }()
	ns, err := c.GetNodes(ctx, podname, nodename, labels, all)
	if err != nil {
		return err
	}

	for _, n := range ns {
		lock, err := c.doLock(ctx, fmt.Sprintf(cluster.NodeLock, podname, n.Name), c.config.LockTimeout)
		if err != nil {
			return err
		}
		log.Debugf("[withNodesLocked] Node %s locked", n.Name)
		locks[n.Name] = lock
		// refresh node
		node, err := c.GetNode(ctx, n.Name)
		if err != nil {
			return err
		}
		nodes[n.Name] = node
	}
	return f(nodes)
}
