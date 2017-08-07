package util

import (
	"errors"
)

type Stats struct {
	Tx_bytes uint64
	Rx_bytes uint64
	Tx_pkts  uint64
	Rx_pkts  uint64
}

const (
	FATAL = iota
)
const (
	MAX_WRITE_RETRY = 5
)

var (
	ErrTooManyRetires = errors.New("too many retries")
)

type StatusChangeElem struct {
	Target string
	Status int
}

const (
	TCP_CREATED = iota
	TCP_RUNNING
	TCP_STOP
)
