package network

import (
	"fmt"
	"github.com/danieldin95/openlan-go/libol"
	"github.com/golang-collections/go-datastructures/queue"
	"sync"
	"time"
)

type Learner struct {
	Dest    []byte
	Device  Taper
	Uptime  int64
	Newtime int64
}

type VirBridge struct {
	mtu      int
	name     string
	inQ      *queue.Queue
	lock     sync.RWMutex
	devices  map[string]Taper
	learners map[string]*Learner
	done     chan bool
	ticker   *time.Ticker
	timeout  int
}

func NewVirBridge(name string, mtu int) *VirBridge {
	b := &VirBridge{
		name:     name,
		mtu:      mtu,
		inQ:      queue.New(1024 * 32),
		devices:  make(map[string]Taper, 1024),
		learners: make(map[string]*Learner, 1024),
		done:     make(chan bool),
		ticker:   time.NewTicker(5 * time.Second),
		timeout:  5 * 60,
	}
	return b
}

func (b *VirBridge) Open(addr string) {
	libol.Info("VirBridge.Open: not support address")
	go b.Start()
}

func (b *VirBridge) Close() error {
	b.inQ.Dispose()
	return nil
}

func (b *VirBridge) AddSlave(dev Taper) error {
	dev.Slave(b)

	b.lock.Lock()
	defer b.lock.Unlock()
	b.devices[dev.Name()] = dev

	libol.Info("VirBridge.AddSlave: %s %s", dev.Name(), b.name)

	return nil
}

func (b *VirBridge) DelSlave(dev Taper) error {
	b.lock.Lock()
	defer b.lock.Unlock()
	if _, ok := b.devices[dev.Name()]; ok {
		delete(b.devices, dev.Name())
	}

	libol.Info("VirBridge.DelSlave: %s %s", dev.Name(), b.name)

	return nil
}

func (b *VirBridge) Name() string {
	return b.name
}

func (b *VirBridge) SetName(value string) {
	b.name = value
}

func (b *VirBridge) SetTimeout(value int) {
	b.timeout = value
}

func (b *VirBridge) Start() {
	forward := func() {
		for {
			result, err := b.inQ.Get(1)
			if err != nil {
				return
			}
			m := result[0].(*Framer)
			if is := b.Unicast(m); !is {
				b.Flood(m)
			}
		}
	}
	expired := func() {
		select {
		case <-b.done:
			return
		case <-b.ticker.C:
			deletes := make([]string, 0, 1024)

			//collect need deleted.
			b.lock.RLock()
			for index, learn := range b.learners {
				now := time.Now().Unix()
				if now-learn.Uptime > int64(b.timeout) {
					deletes = append(deletes, index)
				}
			}
			b.lock.RUnlock()

			//execute delete.
			b.lock.Lock()
			for _, d := range deletes {
				if _, ok := b.learners[d]; ok {
					delete(b.learners, d)
					libol.Info("VirBridge.Start.Ticker: delete %s", d)
				}
			}
			b.lock.Unlock()
		}
	}

	go expired()
	go forward()
}

func (b *VirBridge) Input(m *Framer) error {
	b.Learn(m)
	return b.inQ.Put(m)
}

func (b *VirBridge) Output(m *Framer) error {
	var err error

	libol.Debug("VirBridge.Output: % x", m.Data[:20])
	if dev := m.Output; dev != nil {
		_, err = dev.InRead(m.Data)
	}

	return err
}

func (b *VirBridge) Eth2Str(addr []byte) string {
	if len(addr) < 6 {
		return ""
	}
	return fmt.Sprintf("%02x%02x%02x%02x%02x%02x",
		addr[0], addr[1], addr[2], addr[3], addr[4], addr[5])
}

func (b *VirBridge) Learn(m *Framer) {
	source := m.Data[6:12]
	if source[0]&0x01 == 0x01 {
		return
	}

	index := b.Eth2Str(source)
	if l := b.FindDest(index); l != nil {
		b.UpdateDest(index)
		return
	}

	learn := &Learner{
		Device:  m.Source,
		Uptime:  time.Now().Unix(),
		Newtime: time.Now().Unix(),
	}
	learn.Dest = make([]byte, 6)
	copy(learn.Dest, source)

	libol.Info("VirBridge.Learn: %s on %s", index, m.Source)
	b.AddDest(index, learn)
}

func (b *VirBridge) FindDest(d string) *Learner {
	b.lock.RLock()
	defer b.lock.RUnlock()

	if l, ok := b.learners[d]; ok {
		return l
	}
	return nil
}

func (b *VirBridge) AddDest(d string, l *Learner) {
	b.lock.Lock()
	defer b.lock.Unlock()
	b.learners[d] = l
}

func (b *VirBridge) UpdateDest(d string) {
	b.lock.RLock()
	defer b.lock.RUnlock()

	if l, ok := b.learners[d]; ok {
		l.Uptime = time.Now().Unix()
	}
}

func (b *VirBridge) Flood(m *Framer) error {
	var err error

	libol.Debug("VirBridge.Flood: % x", m.Data[:20])
	for _, dev := range b.devices {
		if m.Source == dev {
			continue
		}

		_, err = dev.InRead(m.Data)
	}
	return err
}

func (b *VirBridge) Unicast(m *Framer) bool {
	data := m.Data
	index := b.Eth2Str(data[:6])

	if l := b.FindDest(index); l != nil {
		dev := l.Device
		if dev != m.Source {
			if _, err := dev.InRead(m.Data); err != nil {
				libol.Debug("VirBridge.Unicast: %s %s", dev, err)
			}
		}
		libol.Debug("VirBridge.Unicast: from %s to %s % x", m.Source, dev, data[:20])
		return true
	}

	return false
}
