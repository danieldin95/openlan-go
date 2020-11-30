package storage

import (
	"bufio"
	"github.com/danieldin95/openlan-go/src/libol"
	"github.com/danieldin95/openlan-go/src/olsw/schema"
	"os"
	"strconv"
	"strings"
	"time"
)

type ovClient struct {
	WorkDir string
}

var OvClient = ovClient{
	WorkDir: "/var/openlan/openvpn/",
}

func ParseInt64(value string) (int64, error) {
	return strconv.ParseInt(value, 10, 64)
}

func (o *ovClient) readStatus(network string) map[string]*schema.OvClient {
	file, err := os.Open(o.WorkDir + network + "/server.status")
	if err != nil {
		libol.Debug("ovClient.readStatus %v", err)
		return nil
	}
	defer file.Close()

	readAt := "header"
	offset := 0
	scanner := bufio.NewScanner(file)
	clients := make(map[string]*schema.OvClient, 32)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "OpenVPN CLIENT LIST" {
			readAt = "common"
			offset = 3
		}
		if line == "ROUTING TABLE" {
			readAt = "routing"
			offset = 2
		}
		if line == "GLOBAL STATS" {
			readAt = "global"
			offset = 1
		}
		if offset > 0 {
			offset -= 1
			continue
		}
		columns := strings.SplitN(line, ",", 5)
		switch readAt {
		case "common":
			if len(columns) == 5 {
				name := columns[0]
				client := &schema.OvClient{
					Name:    columns[0],
					Address: columns[1],
					State:   "success",
				}
				if rxc, err := ParseInt64(columns[2]); err == nil {
					client.RxBytes = rxc
				}
				if txc, err := ParseInt64(columns[3]); err == nil {
					client.TxBytes = txc
				}
				if uptime, err := time.Parse(time.ANSIC, columns[4]); err == nil {
					client.UpTime = uptime.Unix()
					client.AliveTime = time.Now().Unix() - client.UpTime

				}
				clients[name] = client
			}
		case "routing":
			// TODO
			continue
		}
	}
	if err := scanner.Err(); err != nil {
		libol.Warn("ovClient.readStatus %v", err)
		return nil
	}
	return clients
}

func (o *ovClient) List(name string) <-chan *schema.OvClient {
	c := make(chan *schema.OvClient, 128)

	clients := o.readStatus(name)
	go func() {
		for _, v := range clients {
			c <- v
		}
		c <- nil //Finish channel by nil.
	}()

	return c
}