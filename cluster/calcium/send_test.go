package calcium

import (
	"context"
	"io/ioutil"
	"os"
	"testing"

	enginemocks "github.com/projecteru2/core/engine/mocks"
	storemocks "github.com/projecteru2/core/store/mocks"
	"github.com/projecteru2/core/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestSend(t *testing.T) {
	c := NewTestCluster()
	ctx := context.Background()
	tmpfile, err := ioutil.TempFile("", "example")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpfile.Name())
	defer tmpfile.Close()
	opts := &types.SendOptions{
		IDs: []string{"cid"},
		Data: map[string]string{
			"/tmp/1": "nofile",
		},
	}
	store := &storemocks.Store{}
	c.store = store
	// failed by GetContainer
	store.On("GetContainer", mock.Anything, mock.Anything).Return(nil, types.ErrNoETCD).Once()
	ch, err := c.Send(ctx, opts)
	assert.NoError(t, err)
	for r := range ch {
		assert.Error(t, r.Error)
	}
	// failed by no file
	engine := &enginemocks.API{}
	store.On("GetContainer", mock.Anything, mock.Anything).Return(
		&types.Container{Engine: engine}, nil,
	)
	ch, err = c.Send(ctx, opts)
	assert.NoError(t, err)
	for r := range ch {
		assert.Error(t, r.Error)
	}
	// failed by engine
	opts.Data["/tmp/1"] = tmpfile.Name()
	engine.On("VirtualizationCopyTo",
		mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything,
	).Return(types.ErrCannotGetEngine).Once()
	ch, err = c.Send(ctx, opts)
	assert.NoError(t, err)
	for r := range ch {
		assert.Error(t, r.Error)
	}
	// success
	engine.On("VirtualizationCopyTo",
		mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything,
	).Return(nil)
	ch, err = c.Send(ctx, opts)
	assert.NoError(t, err)
	for r := range ch {
		assert.NoError(t, r.Error)
		assert.Equal(t, r.ID, "cid")
		assert.Equal(t, r.Path, "/tmp/1")
	}
}
