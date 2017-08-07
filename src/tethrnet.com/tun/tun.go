package tun

import (
	//"github.com/google/gopacket"
	//"github.com/google/gopacket/layers"
	"errors"
	"fmt"
	"github.com/songgao/water"
	//"net"
	"github.com/qiniu/log"
	"tethrnet.com/util"
)

var (
	WriteErr = errors.New("fail to write tun")
)

type Tun struct {
	name   string
	iface  *water.Interface
	cache  []byte
	stats  util.Stats
	closed bool
}

func NewTun(name string, cache []byte) (*Tun, error) {
	config := water.Config{
		DeviceType: water.TUN,
	}
	config.Name = name
	ifce, err := water.New(config)
	if err != nil {
		log.Error("fail to create Tun interface", name, err)
		return nil, err
	}

	t := &Tun{
		name: name,
	}
	t.iface = ifce
	t.cache = cache
	log.Debug("tunnel created", name)
	return t, nil
}

func (t *Tun) String() string {
	return fmt.Sprintf("tun %s, tx_pkts: %u, tx_bytes:%u, rx_pkts: %u, rx_pkts: %u",
		t.name, t.stats.Tx_pkts, t.stats.Tx_bytes, t.stats.Rx_pkts, t.stats.Rx_bytes)
}

func (t *Tun) GetStats() util.Stats {
	return t.stats
}

func (t *Tun) Close() {
	if t.closed == true {
		return
	}
	log.Warnf("Tun intf %s is closing\n", t.name)
	err := t.iface.Close()
	if err != nil {
		log.Error("fail to close tun", err)
	} else {
		t.closed = true
	}
}

func (t *Tun) Read() ([]byte, error) {
	n, err := t.iface.Read(t.cache)
	if err != nil {
		log.Error("fail to read from Tun intf, will close the Tun intf", t.name, err)
		return t.cache, err
	}
	log.Debugf("Tun Read, %d bytes,%x\n", n, t.cache[:n])
	t.stats.Rx_pkts++
	t.stats.Rx_bytes += uint64(n)
	return t.cache[:n], nil
}

func (t *Tun) Write(data []byte) error {
	//retries := 0
	//total := 0
	//for {
	n, err := t.iface.Write(data)
	if err != nil || n != len(data) {
		log.Warnf("fail to write %d(%d bytes wrote) into tun %v, %v", len(data), n, t.name, err)
		return err
	}
	t.stats.Tx_pkts++
	t.stats.Tx_bytes += uint64(len(data))
	return nil
}
