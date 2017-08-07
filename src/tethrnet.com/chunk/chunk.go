package chunk

import (
	"sync"
)

const (
	MaxInCacheLen = 64000
	MaxChunks     = 100
	InvalidChunk  = MaxChunks + 1
)

type Chunk struct {
	ref  int
	data []byte
	lock sync.Mutex
}

func (c *Chunk) GetRef() int {
	return c.ref
}

func (c *Chunk) GetData() []byte {
	return c.data
}

type chunkMgr struct {
	pool      []Chunk
	availChan chan int
}

var Pool *chunkMgr = NewChunkMgr(MaxChunks, MaxInCacheLen)

func NewChunkMgr(chunkNum int, chunkSize int) *chunkMgr {
	mgr := &chunkMgr{}
	mgr.availChan = make(chan int, chunkNum)
	mgr.pool = make([]Chunk, chunkNum)

	for i := 0; i < chunkNum; i++ {
		mgr.pool[i].data = make([]byte, chunkSize)
		mgr.availChan <- i
	}
	return mgr
}

func (m *chunkMgr) GetChunk(idx int) *Chunk {
	return &(m.pool[idx])
}

func (m *chunkMgr) AllocChunk() (int, []byte) {
	idx := <-m.availChan
	m.pool[idx].ref = 1
	return idx, m.pool[idx].data
}

func (m *chunkMgr) RefChunk(idx int) {
	m.pool[idx].lock.Lock()
	m.pool[idx].ref++
	m.pool[idx].lock.Unlock()
}

func (m *chunkMgr) DeRefChunk(idx int) {
	if idx == InvalidChunk {
		return
	}

	m.pool[idx].lock.Lock()
	if m.pool[idx].ref <= 0 {
		m.pool[idx].ref = 0
		m.pool[idx].lock.Unlock()
		return
	}
	m.pool[idx].ref--
	if m.pool[idx].ref == 0 {
		m.availChan <- idx
	}
	m.pool[idx].lock.Unlock()
}
