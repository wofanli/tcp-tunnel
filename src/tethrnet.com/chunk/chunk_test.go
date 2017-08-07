package chunk

import (
	"github.com/stretchr/testify/assert"
	"testing"
	"tethrnet.com/log"
)

func TestChunk(t *testing.T) {
	log.SetOutputLevel(log.Ldebug)
	log.Info("testing Chunk")
	id, data := Pool.AllocChunk()
	assert.Equal(t, MaxInCacheLen, len(data))
	assert.Equal(t, Pool.pool[id].ref, 1)
	Pool.RefChunk(id)
	assert.Equal(t, Pool.pool[id].ref, 2)
	Pool.DeRefChunk(id)
	assert.Equal(t, Pool.pool[id].ref, 1)
	Pool.DeRefChunk(id)
	assert.Equal(t, Pool.pool[id].ref, 0)
	Pool.DeRefChunk(id)
	assert.Equal(t, Pool.pool[id].ref, 0)
}
