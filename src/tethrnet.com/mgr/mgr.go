package mgr

import (
	"C"
	"bytes"
	"errors"
	"github.com/qiniu/log"
	"net"
	"strings"
	"sync"
	hs "tethrnet.com/handshaker"
	"tethrnet.com/tcp"
	"tethrnet.com/util"
	"time"
	"unsafe"
)

const (
	ServerPort = "9999"
)

type targetType struct {
	tcp       *tcp.Tcp
	lock      sync.Mutex
	localName string
	rmtName   string
	state     int
}

var (
	ErrExisting = errors.New("existing")
	NilTcpPtr   = errors.New("nil tcp ptr")
)

func (t *targetType) Start() error {
	if t.tcp == nil {
		return NilTcpPtr
	}
	t.state = util.TCP_RUNNING
	t.tcp.Start()
	return nil
}

func (t *targetType) Stop() {
	t.state = util.TCP_STOP
	if t.tcp != nil {
		t.tcp.Close()
	}
}

type Mgr struct {
	l           net.Listener
	targets     map[string]*targetType
	lock        sync.Mutex
	notify      chan *util.StatusChangeElem
	auditSignal chan bool
}

func NewMgr(localAdr string) *Mgr {
	var err error
	l, err := net.Listen("tcp", localAdr+":"+ServerPort)
	if err != nil {
		log.Fatal(err)
	}
	return &Mgr{
		l:           l,
		auditSignal: make(chan bool, 1),
		targets:     make(map[string]*targetType),
		notify:      make(chan *util.StatusChangeElem, 1000),
	}
}

func (m *Mgr) chkNotifyRun() {
	for msg := range m.notify {
		switch msg.Status {
		case util.TCP_STOP:
			log.Info("got TCP_STOP notify", msg.Target)
			m.delTarget(msg.Target)
		default:
			log.Error("got an invalid notify msg", msg.Status)
		}
	}

}

func (m *Mgr) Run() {
	defer m.l.Close()
	go m.AuditRun()
	for {
		conn, err := m.l.Accept()
		if err != nil {
			log.Error("fail to accept a tcp conn", err)
			continue
		}
		log.Debug("new incoming conn", conn.RemoteAddr().String())
		k := conn.RemoteAddr().String()
		k = strings.Split(k, ":")[0]
		m.lock.Lock()
		target, ok := m.targets[k]
		m.lock.Unlock()
		if ok {
			go m.connHandle(false, k, target, conn)
		} else {
			log.Info("incoming conn for unexisting target", k)
		}
	}
}

func (m *Mgr) reject(conn net.Conn, rmtAdr string) error {
	data := make([]byte, hs.HsHdrLen)
	hdr := (*hs.HandShakeHdr)(unsafe.Pointer(&data[0]))
	hdr.Opt = hs.OptReject
	copy(hdr.Key[:], hs.KEY)
	tcp.WriteToConn(conn, data)
	return conn.Close()
}

func (m *Mgr) delTarget(rmtAdr string) {
	t, ok := m.targets[rmtAdr]
	if ok {
		t.Stop()
		delete(m.targets, rmtAdr)
	}
}

func (m *Mgr) DelTunnel(rmtAdr string) {
	m.lock.Lock()
	m.delTarget(rmtAdr)
	m.lock.Unlock()
}

func (m *Mgr) CreateTunnel(localName, rmtName string, rmtAdr string) error {
	m.lock.Lock()
	defer m.lock.Unlock()
	_, ok := m.targets[rmtAdr]
	if ok {
		return ErrExisting
	}

	t := &targetType{
		localName: localName,
		rmtName:   rmtName,
		state:     util.TCP_CREATED,
	}
	m.targets[rmtAdr] = t
	select {
	case m.auditSignal <- true:
	default:
	}
	return nil
}

func (m *Mgr) AuditRun() {
	ticker := time.NewTicker(time.Second * 10)
	for {
		select {
		case <-ticker.C:
		case <-m.auditSignal:
		}
		log.Debug("mgr AuditRun")
		m.lock.Lock()
		for k, t := range m.targets {
			if t.tcp != nil && t.state == util.TCP_RUNNING && t.tcp.GetState() == util.TCP_RUNNING {
				continue
			}
			if t.localName < t.rmtName {
				//we are the initiator
				log.Debug("we are the initiator, try to connect to ", k)
				conn, err := net.Dial("tcp", k+":"+ServerPort)
				if err != nil {
					log.Error("fail to dial tcp ", k, err)
					continue
				}
				m.connHandle(true, k, t, conn)
			}
		}
		m.lock.Unlock()
	}
}

func (m *Mgr) init(t *targetType, conn net.Conn) error {
	data := make([]byte, hs.HsHdrLen)
	hdr := (*hs.HandShakeHdr)(unsafe.Pointer(&data[0]))
	hdr.Opt = hs.OptInit
	copy(hdr.LocalName[:], []byte(t.localName))
	copy(hdr.RmtName[:], []byte(t.rmtName))
	copy(hdr.Key[:], hs.KEY)
	log.Debug("on sending init", data, hdr)
	err := tcp.WriteToConn(conn, data)
	if err != nil {
		log.Error("fail to send init msg", conn.RemoteAddr(), err)
		return err
	} else {
		log.Debug("write init msg", conn.RemoteAddr(), hs.OptInit)
	}
	return nil
}

func (m *Mgr) accept(t *targetType, conn net.Conn) error {
	data := make([]byte, hs.HsHdrLen)
	hdr := (*hs.HandShakeHdr)(unsafe.Pointer(&data[0]))
	hdr.Opt = hs.OptAccept
	copy(hdr.LocalName[:], []byte(t.localName))
	copy(hdr.RmtName[:], []byte(t.rmtName))
	copy(hdr.Key[:], hs.KEY)
	err := tcp.WriteToConn(conn, data)
	if err != nil {
		log.Error("fail to send accept msg", conn.RemoteAddr(), err)
		return err
	}
	return nil
}

func (m *Mgr) connHandle(is_init bool, rmtAdr string, target *targetType, conn net.Conn) {
	var err error
	pktChan := tcp.DecodeRcvPkt(conn)
	target.lock.Lock()
	defer target.lock.Unlock()

	target.tcp, err = tcp.NewTcp(conn, rmtAdr, target.rmtName, m.notify, pktChan)
	if err != nil {
		log.Error("fail to create tcp inst", err)
		conn.Close()
		return
	}

	if is_init == true {
		err = m.init(target, conn)
		if err != nil {
			log.Error("fail to send out init msg", err)
			return
		}
	}

	pkt := <-pktChan
	if pkt == nil {
		log.Error("socket ends during handshake")
		return
	}
	defer pkt.Free()
	log.Debug("got pkt from ", rmtAdr, pkt.Opt)
	if pkt.Opt != hs.OptInit && pkt.Opt != hs.OptAccept {
		log.Warn("got invalid msg during handshake", pkt.Opt)
		conn.Close()
		return
	}
	hdr := (*hs.HandShakeHdr)(unsafe.Pointer(&pkt.Data[0]))
	if bytes.Equal(hdr.Key[:], hs.KEY) != true {
		log.Error("got an invalid msg with wrong key", string(hdr.Key[:]))
		conn.Close()
		return
	}

	if pkt.Opt == hs.OptInit {
		if target.state != util.TCP_CREATED {
			log.Info("reject incoming req", rmtAdr, target.state)
			m.reject(conn, rmtAdr)
			return
		} else {
			log.Info("accepting incoming req", rmtAdr)
			err := m.accept(target, conn)
			if err != nil {
				log.Error("fail to send out accept message", rmtAdr, err)
				conn.Close()
				return
			}
		}
	}
	log.Info("starting target", target.rmtName)
	err = target.Start()
	if err != nil {
		log.Error("fail to start target", target.localName, target.rmtName, err)
	}
}
