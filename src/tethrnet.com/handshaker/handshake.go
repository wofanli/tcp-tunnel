package handshaker

import (
	"tethrnet.com/chunk"
	"unsafe"
)

const (
	OptData = iota
	OptInit
	OptAccept
	OptReject
	OptKeepAlive
)

type HdrBase struct {
	Opt uint32
}

var HdrBaseLen = int(unsafe.Sizeof(HdrBase{}))

type DataHdr struct {
	HdrBase
	Len uint32
}

var DataHdrLen = int(unsafe.Sizeof(DataHdr{}))

var (
	KEY = []byte("LoVeEnDuRes4Ever")
)

const (
	KEY_SIZE = 16 // length of KEY string
)

const NameLen = 16

type HandShakeHdr struct {
	HdrBase
	LocalName [NameLen]byte
	RmtName   [NameLen]byte
	Key       [KEY_SIZE]byte
}

var HsHdrLen = int(unsafe.Sizeof(HandShakeHdr{}))

type PktType struct {
	Opt      uint32
	Data     []byte
	ChunkIdx int
}

func NewPkt(opt uint32, data []byte, idx int) *PktType {
	pkt := &PktType{
		Opt:      opt,
		Data:     data,
		ChunkIdx: idx,
	}
	chunk.Pool.RefChunk(idx)
	return pkt
}

func (p *PktType) Free() {
	chunk.Pool.DeRefChunk(p.ChunkIdx)
}
