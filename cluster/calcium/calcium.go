package calcium

import (
	"strings"

	"github.com/projecteru2/core/cluster"
	"github.com/projecteru2/core/scheduler"
	complexscheduler "github.com/projecteru2/core/scheduler/complex"
	"github.com/projecteru2/core/source"
	"github.com/projecteru2/core/source/github"
	"github.com/projecteru2/core/source/gitlab"
	"github.com/projecteru2/core/store"
	"github.com/projecteru2/core/store/etcdv3"
	"github.com/projecteru2/core/types"
	log "github.com/sirupsen/logrus"
)

//Calcium implement the cluster
type Calcium struct {
	config    types.Config
	store     store.Store
	scheduler scheduler.Scheduler
	source    source.Source
}

// New returns a new cluster config
func New(config types.Config, embededStorage bool) (*Calcium, error) {
	// set store
	store, err := etcdv3.New(config, embededStorage)
	if err != nil {
		return nil, err
	}

	// set scheduler
	scheduler, err := complexscheduler.New(config)
	if err != nil {
		return nil, err
	}

	// set scm
	var scm source.Source
	scmtype := strings.ToLower(config.Git.SCMType)
	switch scmtype {
	case cluster.Gitlab:
		scm = gitlab.New(config)
	case cluster.Github:
		scm = github.New(config)
	default:
		log.Warn("[Calcium] SCM not set, build API disabled")
	}

	return &Calcium{store: store, config: config, scheduler: scheduler, source: scm}, nil
}

// Finalizer use for defer
func (c *Calcium) Finalizer() {
	c.store.TerminateEmbededStorage()
}
