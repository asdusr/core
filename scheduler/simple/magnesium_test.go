package simplescheduler

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"gitlab.ricebook.net/platform/core/types"
)

func TestRandomNode(t *testing.T) {
	m := &magnesium{}
	_, err := m.RandomNode(map[string]types.CPUMap{})
	assert.Error(t, err)
	assert.Equal(t, err.Error(), "No nodes provide to choose one")

	nodes := map[string]types.CPUMap{
		"node1": types.CPUMap{
			"0": 10,
			"1": 10,
		},
		"node2": types.CPUMap{
			"0": 10,
			"1": 10,
		},
	}

	node, err := m.RandomNode(nodes)
	assert.NoError(t, err)
	assert.Contains(t, []string{"node1", "node2"}, node)

	nodes = map[string]types.CPUMap{
		"node1": types.CPUMap{
			"0": 10,
			"1": 10,
		},
	}

	node, err = m.RandomNode(nodes)
	assert.NoError(t, err)
	assert.Equal(t, "node1", node)
}

func TestSelectCPUNodes(t *testing.T) {
	m := &magnesium{}
	_, _, err := m.SelectCPUNodes([]types.NodeInfo{}, 1, 1)
	assert.Error(t, err)
	assert.Equal(t, err.Error(), "No nodes provide to choose some")

	nodes := []types.NodeInfo{
		types.NodeInfo{
			types.CPUAndMem{
				types.CPUMap{
					"0": 10,
					"1": 10,
				},
				12400000,
			},
			"node1", 0.0, 0, 0, 0,
		},
		types.NodeInfo{
			types.CPUAndMem{
				types.CPUMap{
					"0": 10,
					"1": 10,
				},
				12400000,
			},
			"node2", 0.0, 0, 0, 0,
		},
	}

	_, _, err = m.SelectCPUNodes(nodes, 2, 3)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Not enough")

	_, _, err = m.SelectCPUNodes(nodes, 3, 2)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Not enough")

	_, _, err = m.SelectCPUNodes(nodes, 1, 5)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Not enough")

	r, changed, err := m.SelectCPUNodes(nodes, 1, 2)
	assert.NoError(t, err)
	assert.Equal(t, len(changed), 2)
	for nodename, cpus := range r {
		assert.Contains(t, []string{"node1", "node2"}, nodename)
		assert.Equal(t, len(cpus), 1)

		cpu := cpus[0]
		assert.Equal(t, int(cpu.Total()), 10)
	}

	_, _, err = m.SelectCPUNodes(nodes, 1, 4)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Not enough")

	r, _, err = m.SelectCPUNodes(nodes, 1, 2)
	assert.NoError(t, err)
	for nodename, cpus := range r {
		assert.Contains(t, []string{"node1", "node2"}, nodename)
		assert.Equal(t, len(cpus), 1)

		cpu := cpus[0]
		assert.Equal(t, int(cpu.Total()), 10)
	}

	_, _, err = m.SelectCPUNodes(nodes, 1, 1)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Not enough")

	for _, node := range nodes {
		cupmap := node.CpuMap
		assert.Equal(t, int(cupmap.Total()), 0)
		assert.Equal(t, len(cupmap), 2)
		assert.Equal(t, int(cupmap["0"]), 0)
		assert.Equal(t, int(cupmap["1"]), 0)
	}
}

func TestResultLength(t *testing.T) {
	c := map[string][]types.CPUMap{
		"node1": []types.CPUMap{
			types.CPUMap{"0": 10},
			types.CPUMap{"1": 10},
		},
		"node2": []types.CPUMap{
			types.CPUMap{"0": 10},
		},
	}
	assert.Equal(t, resultLength(c), 3)
}

func TestTotalQuota(t *testing.T) {
	nodes := []types.NodeInfo{
		types.NodeInfo{
			types.CPUAndMem{
				types.CPUMap{
					"0": 10,
					"1": 0,
				},
				12400000,
			},
			"node1", 0.0, 0, 0, 0,
		},
		types.NodeInfo{
			types.CPUAndMem{
				types.CPUMap{
					"0": 5,
					"1": 10,
				},
				12400000,
			},
			"node2", 0.0, 0, 0, 0,
		},
	}
	assert.Equal(t, totalQuota(nodes), 3)
}

func TestSelectPublicNodes(t *testing.T) {
	m := &magnesium{}
	_, _, err := m.SelectCPUNodes([]types.NodeInfo{}, 1, 1)
	assert.Error(t, err)
	assert.Equal(t, err.Error(), "No nodes provide to choose some")

	nodes := []types.NodeInfo{
		types.NodeInfo{
			types.CPUAndMem{
				types.CPUMap{
					"0": 10,
					"1": 10,
				},
				12400000,
			},
			"node1", 0.0, 0, 0, 0,
		},
		types.NodeInfo{
			types.CPUAndMem{
				types.CPUMap{
					"0": 10,
					"1": 10,
				},
				12400000,
			},
			"node2", 0.0, 0, 0, 0,
		},
	}

	r, changed, err := m.SelectCPUNodes(nodes, 0, 10)
	assert.NoError(t, err)
	assert.Equal(t, resultLength(r), 10)
	assert.Equal(t, len(changed), 2)
	for nodename, cpus := range r {
		assert.Contains(t, []string{"node1", "node2"}, nodename)
		for _, cpu := range cpus {
			assert.Equal(t, int(cpu.Total()), 0)
		}
	}

	for _, node := range nodes {
		nodename := node.Name
		assert.Contains(t, []string{"node1", "node2"}, nodename)
		assert.Equal(t, int(node.CpuMap.Total()), 20)
	}
}
