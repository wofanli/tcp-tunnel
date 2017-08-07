package tcp

import (
	"C"
	"github.com/qiniu/log"
	"net"
	"tethrnet.com/chunk"
	hs "tethrnet.com/handshaker"
	"tethrnet.com/tun"
	"tethrnet.com/util"
	"time"
	"unsafe"
)

const (
	MaxOutCacheLen = 2000
	MaxInCacheLen  = 6400000
)

var hostname string
var HostnameByte []byte

/*
 * The message is formated with:
 * | Opt (1byte) | Para (4 byte) | data ...
 */

const (
	KeepAlive = time.Second * 1
)

type Tcp struct {
	conn      net.Conn
	iface     *tun.Tun
	startTs   time.Time
	touch     bool
	state     int
	target    string
	notify    chan *util.StatusChangeElem
	stats     util.Stats
	outCache  []byte
	inPktChan chan *hs.PktType
}

func NewTcp(c net.Conn, target string, name string, notify chan *util.StatusChangeElem, pktC chan *hs.PktType) (*Tcp, error) {
	outCache := make([]byte, MaxOutCacheLen)
	iface, err := tun.NewTun(name, outCache[hs.DataHdrLen:])
	if err != nil {
		log.Debug("NewTcp fails", target, name, err)
		return nil, err
	}
	t := &Tcp{
		conn:      c,
		iface:     iface,
		startTs:   time.Now(),
		state:     util.TCP_CREATED,
		notify:    notify,
		outCache:  outCache,
		inPktChan: pktC,
	}
	log.Debug("NewTcp success", target, name)
	return t, nil
}

func (t *Tcp) GetState() int {
	return t.state
}

func (t *Tcp) Start() {
	t.state = util.TCP_RUNNING
	go t.proxyRcvRun()
	go t.proxyForwardRun()
	go t.keepAlive()
}

func (t *Tcp) Close() {
	log.Warn("tcp conn is closing", t.target)
	t.iface.Close()
	t.conn.Close()
	t.state = util.TCP_STOP
}

func (t *Tcp) toClose() {
	t.Close()
	elem := &util.StatusChangeElem{
		Target: t.target,
		Status: util.TCP_STOP,
	}
	t.notify <- elem
}

func WriteToConn(conn net.Conn, data []byte) error {
	retry := 0
	sum := 0
	for {
		n, err := conn.Write(data[sum:])
		if err != nil {
			return err
		}
		sum += n
		if sum >= len(data) {
			return nil
		}
		log.Debug("tcp WriteToConn retry", conn.RemoteAddr())
		retry++
		if retry >= util.MAX_WRITE_RETRY {
			return util.ErrTooManyRetires
		}
	}
	return nil
}

func (t *Tcp) WriteToConn(data []byte) error {
	err := WriteToConn(t.conn, data)
	if err != nil {
		return err
	}
	t.stats.Tx_pkts++
	t.stats.Tx_bytes += uint64(len(data))
	return nil
}

func (t *Tcp) proxyForwardRun() {
	hdr := (*hs.DataHdr)(unsafe.Pointer(&t.outCache[0]))
	for {
		if t.state == util.TCP_STOP {
			return
		}
		frame, err := t.iface.Read()
		if err != nil {
			log.Warn("tcp read tun fails, will close it", t.target, err)
			t.toClose()
			return
		}
		hdr.Opt = hs.OptData
		hdr.Len = uint32(len(frame))
		data := t.outCache[:hs.DataHdrLen+len(frame)]
		log.Debugf("proxyForwardRun, ford pkt through tcp tunnel:%v, %d, %x\n", t.conn.RemoteAddr(), hdr.Len, data)
		err = t.WriteToConn(data)
		if err != nil {
			log.Warn("tcp fail to write data onto wire", err)
			t.toClose()
			return
		}
	}
}

const (
	READ_CONTROL_HDR = iota
	READ_DATA
)

func DecodeRcvPkt(conn net.Conn) chan *hs.PktType {
	pktChan := make(chan *hs.PktType, 1000)
	go func() {
		i := 0
		n := 0
		length := 0
		var err error
		chunkIdx := chunk.InvalidChunk
		oldChunkIdx := chunk.InvalidChunk
		var data, oldData []byte
		for {
			oldData = data
			oldChunkIdx = chunkIdx
			chunkIdx, data = chunk.Pool.AllocChunk()
			if i < length {
				log.Info("broken pkt, need to cache it ", i, length)
				copy(data, oldData[i:length])
			}
			length = length - i

			chunk.Pool.DeRefChunk(oldChunkIdx)
			n, err = conn.Read(data[length:])
			if err != nil {
				log.Warn("tcp read data from wired fails", err)
				close(pktChan)
				chunk.Pool.DeRefChunk(chunkIdx)
				return
			}
			length += n
			log.Debugf("got %d bytes from %v, %x\n", length, conn.RemoteAddr(), data[:length])
			i = 0
		outLoop:
			for {
				log.Debug("grap one packet from incoming stream")
				if i+hs.HdrBaseLen > length {
					break
				}
				hdr := (*hs.HdrBase)(unsafe.Pointer(&data[i]))
				switch hdr.Opt {
				case hs.OptInit, hs.OptKeepAlive, hs.OptAccept, hs.OptReject:
					end := i + hs.HsHdrLen
					if n < end {
						break outLoop
					}
					log.Debugf("got hs pkt from wired, src: %v, opt:%d, start_idx:%d, end_idx:%d, %v\n",
						conn.RemoteAddr(), hdr.Opt, i, end, data[i:end])
					pktChan <- hs.NewPkt(hdr.Opt, data[i:end], chunkIdx)
					i = end
				case hs.OptData:
					if i+hs.DataHdrLen > length {
						break outLoop
					}
					dataHdr := (*hs.DataHdr)(unsafe.Pointer(&data[i]))
					end := dataHdr.Len + uint32(i) + uint32(hs.DataHdrLen)
					if end > uint32(length) {
						break outLoop
					}
					log.Debugf("got data pkt from wired, src: %v, opt:%d, start_idx:%d, end_idx:%d, %x\n",
						conn.RemoteAddr(), hdr.Opt, i, end, data[i:end])
					pktChan <- hs.NewPkt(hdr.Opt, data[i:end], chunkIdx)
					i = int(end)
				default:
					log.Error("invalid stream got", hdr.Opt, hs.OptAccept, hs.OptAccept == hdr.Opt)
					i = length
					break outLoop
				}
			}
		}
	}()
	return pktChan
}

func (t *Tcp) proxyRcvRun() {
	for pkt := range t.inPktChan {
		if t.state == util.TCP_STOP {
			t.toClose()
			pkt.Free()
			return
		}
		hdr := (*hs.DataHdr)(unsafe.Pointer(&pkt.Data[0]))
		log.Debugf("proxyRcvRun, got pkt:%x(%d bytes),len:%d\n", pkt.Data, len(pkt.Data), hdr.Len)
		switch hdr.Opt {
		case hs.OptData:
			err := t.iface.Write(pkt.Data[hs.DataHdrLen:])
			if err != nil {
				log.Error("fail to write data onto tun ", err)
				t.toClose()
				pkt.Free()
				return
			}
		case hs.OptKeepAlive:
		default:
			log.Errorf("invalid stream got, %d, %x\n", hdr.Opt, pkt.Data)
			t.toClose()
			pkt.Free()
			return
		}
		t.touch = true
		t.stats.Rx_pkts += 1
		t.stats.Rx_bytes += uint64(len(pkt.Data))
		pkt.Free()
		log.Debug("proxyRcvRun, done")
	}
}

func (t *Tcp) SendControlMsg(opt uint32) error {
	data := make([]byte, hs.HsHdrLen)
	hdr := (*hs.HandShakeHdr)(unsafe.Pointer(&data[0]))
	hdr.Opt = opt
	err := t.WriteToConn(data)
	if err != nil {
		log.Error("fail to write onto wire", err)
		t.toClose()
	}
	return nil
}

func (t *Tcp) keepAlive() {
	ticker := time.NewTicker(KeepAlive)
	for {
		if t.state == util.TCP_STOP {
			return
		}
		<-ticker.C
		if t.touch == true {
			log.Info("keepAlive checking with touch true")
			t.touch = false
		} else {
			log.Info("keepAlive checking with touch false")
			t.SendControlMsg(hs.OptKeepAlive)
		}
	}
}
