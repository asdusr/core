package etcdv3

import (
	"testing"

	"github.com/projecteru2/core/types"
	"github.com/stretchr/testify/assert"
)

func TestParseStatusKey(t *testing.T) {
	key := "/deploy/appname/entry/node/id"
	p1, p2, p3, p4 := parseStatusKey(key)
	assert.Equal(t, p1, "appname")
	assert.Equal(t, p2, "entry")
	assert.Equal(t, p3, "node")
	assert.Equal(t, p4, "id")
}

func TestSetCount(t *testing.T) {
	nodesCount := map[string]int{
		"n1": 1,
		"n2": 2,
	}
	nodesInfo := []types.NodeInfo{
		{Name: "n1"},
		{Name: "n2"},
	}
	nodesInfo = setCount(nodesCount, nodesInfo)
	assert.Equal(t, nodesInfo[0].Count, 1)
	assert.Equal(t, nodesInfo[1].Count, 2)
}
