package hardware

import (
	"encoding/json"
	"net"
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/pkg/errors"
	"inet.af/netaddr"
)

type id string
type mac string

// Hardware is the interface to the in memory DB of hardware objects
type Hardware struct {
	mu sync.RWMutex
	hw map[id]struct {
		j    string
		ips  map[netaddr.IP]bool
		macs map[mac]bool
	}
	byIP  map[netaddr.IP]id
	byMAC map[mac]id
}

type hardware struct {
	ID       string
	State    string
	Instance struct {
		IPs []struct {
			Address string
		} `json:"ip_addresses"`
	}
	IPs []struct {
		Address string
	} `json:"ip_addresses"`
	Ports []struct {
		Data struct {
			MAC string
		}
	} `json:"network_ports"`
}

// New will return an initialized Hardware struct
func New() *Hardware {
	return &Hardware{
		hw: map[id]struct {
			j    string
			ips  map[netaddr.IP]bool
			macs map[mac]bool
		}{},
		byIP:  map[netaddr.IP]id{},
		byMAC: map[mac]id{},
	}
}

// Add inserts a new hardware object into the database, overriding any pre-existing values.
// If state == deleted Add will delete the the object from the db.
func (h *Hardware) Add(j string) (string, error) {
	hw := hardware{}
	err := json.Unmarshal([]byte(j), &hw)
	if err != nil {
		return "", errors.Wrap(err, "unable to decode json")
	}
	id := id(hw.ID)
	if _, err = uuid.Parse(hw.ID); err != nil {
		return "", errors.Wrap(err, "not a valid uuid for id")
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	og := h.hw[id]
	ng := h.hw[id]
	ng.j = j
	ng.ips = map[netaddr.IP]bool{}
	ng.macs = map[mac]bool{}

	if hw.State == "deleted" {
		hw.IPs = nil
		hw.Instance.IPs = nil
		hw.Ports = nil
	}

	for _, ip := range hw.IPs {
		nIP, ok := netaddr.FromStdIP(net.ParseIP(ip.Address))
		if !ok {
			return "", errors.New("failed to parse ip")
		}

		if _, ok := og.ips[nIP]; ok {
			og.ips[nIP] = false
		}
		ng.ips[nIP] = true
		h.byIP[nIP] = id

	}
	for _, ip := range hw.Instance.IPs {
		nIP, ok := netaddr.FromStdIP(net.ParseIP(ip.Address))
		if !ok {
			return "", errors.New("failed to parse ip")
		}

		if _, ok := og.ips[nIP]; ok {
			og.ips[nIP] = false
		}
		ng.ips[nIP] = true
		h.byIP[nIP] = id
	}
	for ip, del := range og.ips {
		if del {
			if h.byIP[ip] == id {
				delete(h.byIP, ip)
			}
		}
	}

	for _, port := range hw.Ports {
		m, err := net.ParseMAC(port.Data.MAC)
		if err != nil {
			return "", errors.Wrap(err, "failed to parse mac")
		}
		mac := mac(m.String())

		if _, ok := og.macs[mac]; ok {
			og.macs[mac] = false
		}
		ng.macs[mac] = true
		h.byMAC[mac] = id
	}
	for mac, del := range og.macs {
		if del {
			if h.byMAC[mac] == id {
				delete(h.byMAC, mac)
			}
		}
	}

	if hw.State != "deleted" {
		h.hw[id] = ng
	} else {
		delete(h.hw, id)
	}

	return string(id), nil
}

// All returns each entry stored in memory
func (h *Hardware) All(fn func(string) error) error {
	hw := map[id]string{}
	h.mu.RLock()
	for k, v := range h.hw {
		hw[k] = v.j
	}
	h.mu.RUnlock()

	for _, v := range hw {
		err := fn(v)
		if err != nil {
			return errors.Wrap(err, "callback function returned an error")
		}
	}
	return nil
}

// ByID returns the hardware with the given id
func (h *Hardware) ByID(v string) (string, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	v = strings.TrimSpace(strings.ToLower(v))
	return h.hw[id(v)].j, nil

}

// ByID returns the hardware with the given ip address
func (h *Hardware) ByIP(v string) (string, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	ip, ok := netaddr.FromStdIP(net.ParseIP(v))
	if !ok {
		return "", errors.New("failed to parse ip")
	}

	return h.hw[h.byIP[ip]].j, nil

}

// ByID returns the hardware with the given mac address
func (h *Hardware) ByMAC(v string) (string, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	m, err := net.ParseMAC(strings.TrimSpace(strings.ToLower(v)))
	if err != nil {
		return "", errors.Wrap(err, "failed to parse mac")
	}
	id := h.byMAC[mac(m.String())]
	return h.hw[id].j, nil
}
