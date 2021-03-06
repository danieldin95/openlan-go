package store

import (
	"encoding/binary"
	"github.com/danieldin95/openlan-go/src/libol"
	"github.com/danieldin95/openlan-go/src/models"
	"github.com/danieldin95/openlan-go/src/schema"
	"net"
)

type _network struct {
	Networks *libol.SafeStrMap
	UUID     *libol.SafeStrMap // TODO with network
	Addr     *libol.SafeStrMap // TODO with network
}

func (w *_network) Add(n *models.Network) {
	libol.Debug("_network.Add %v", *n)
	_ = w.Networks.Set(n.Name, n)
}

func (w *_network) Del(name string) {
	libol.Debug("_network.Del %s", name)
	w.Networks.Del(name)
}

func (w *_network) Get(name string) *models.Network {
	if v := w.Networks.Get(name); v != nil {
		return v.(*models.Network)
	}
	return nil
}

//TODO add/del route

func (w *_network) List() <-chan *models.Network {
	c := make(chan *models.Network, 128)

	go func() {
		w.Networks.Iter(func(k string, v interface{}) {
			c <- v.(*models.Network)
		})
		c <- nil //Finish channel by nil.
	}()
	return c
}

func (w *_network) ListLease() <-chan *schema.Lease {
	c := make(chan *schema.Lease, 128)

	go func() {
		w.UUID.Iter(func(k string, v interface{}) {
			c <- v.(*schema.Lease)
		})
		c <- nil //Finish channel by nil.
	}()
	return c
}

func (w *_network) allocLease(sAddr, eAddr string) string {
	sIp := net.ParseIP(sAddr)
	eIp := net.ParseIP(eAddr)
	if sIp == nil || eIp == nil {
		return ""
	}
	start := binary.BigEndian.Uint32(sIp.To4()[:4])
	end := binary.BigEndian.Uint32(eIp.To4()[:4])
	for i := start; i <= end; i++ {
		tmp := make([]byte, 4)
		binary.BigEndian.PutUint32(tmp[:4], i)
		tmpStr := net.IP(tmp).String()
		if _, ok := w.Addr.GetEx(tmpStr); !ok {
			return tmpStr
		}
	}
	return ""
}

func (w *_network) NewLease(uuid, network string) *schema.Lease {
	n := w.Get(network)
	if n == nil || uuid == "" {
		return nil
	}
	if obj, ok := w.UUID.GetEx(uuid); ok {
		l := obj.(*schema.Lease)
		return l // how to resolve conflict with new point?.
	}
	ipStr := w.allocLease(n.IpStart, n.IpEnd)
	if ipStr == "" {
		return nil
	}
	w.AddLease(uuid, ipStr)
	return w.GetLease(uuid)
}

func (w *_network) GetLease(uuid string) *schema.Lease {
	if obj, ok := w.UUID.GetEx(uuid); ok {
		return obj.(*schema.Lease)
	}
	return nil
}

func (w *_network) GetLeaseByAlias(name string) *schema.Lease {
	if obj, ok := w.UUID.GetEx(name); ok {
		return obj.(*schema.Lease)
	}
	return nil
}

func (w *_network) AddLease(uuid, ipStr string) *schema.Lease {
	libol.Info("_network.AddLease %s %s", uuid, ipStr)
	if ipStr != "" {
		l := &schema.Lease{
			UUID:    uuid,
			Alias:   uuid,
			Address: ipStr,
		}
		_ = w.UUID.Set(uuid, l)
		_ = w.Addr.Set(ipStr, l)
		return l
	}
	return nil
}

func (w *_network) DelLease(uuid string) {
	libol.Debug("_network.DelLease %s", uuid)
	// TODO record free address for alias and wait timeout to release.
	addr := ""
	if obj, ok := w.UUID.GetEx(uuid); ok {
		lease := obj.(*schema.Lease)
		addr = lease.Address
		libol.Info("_network.DelLease (%s, %s) by UUID", uuid, addr)
		if lease.Type != "static" {
			w.UUID.Del(uuid)
		}
	}
	if obj, ok := w.Addr.GetEx(addr); ok {
		lease := obj.(*schema.Lease)
		if lease.UUID == uuid { // avoid address conflict by different points.
			libol.Info("_network.DelLease (%s, %s) by Addr", uuid, addr)
			if lease.Type != "static" {
				w.Addr.Del(addr)
			}
		}
	}
}

var Network = _network{
	Networks: libol.NewSafeStrMap(1024),
	UUID:     libol.NewSafeStrMap(1024),
	Addr:     libol.NewSafeStrMap(1024),
}
