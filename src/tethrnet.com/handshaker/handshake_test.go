package handshaker

import (
	"github.com/stretchr/testify/assert"
	"testing"
	"tethrnet.com/chunk"
)

func TestNewPkt(t *testing.T) {
	idx, data := chunk.Pool.AllocChunk()
	ref1 := chunk.Pool.GetChunk(idx).GetRef()
	NewPkt(1, data, idx)
	ref2 := chunk.Pool.GetChunk(idx).GetRef()
	assert.Equal(t, ref1+1, ref2)
}
