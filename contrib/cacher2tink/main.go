package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/tinkerbell/boots/packet"
)

const usage = `usage: cacher2tink

Takes cacherc output on stdin, transforms into tinkerbell data model on stdout.


Example:
cacherc -f dc13 mac 1c:34:da:42:b8:34 | ./cacher2tink >tink.json
`

func main() {
	if len(os.Args) > 1 {
		fmt.Fprintln(os.Stderr, usage)
		if len(os.Args) == 2 && (os.Args[1] == "-h" || os.Args[1] == "--help") {
			os.Exit(0)
		}
		os.Exit(1)
	}

	dec := json.NewDecoder(os.Stdin)
	for {
		var m map[string]interface{}
		err := dec.Decode(&m)
		if err != nil {
			if err == io.EOF {
				return
			}
			panic(err)
		}
		if m["instance"] == nil {
			m["instance"] = map[string]interface{}{}
		}
		instance := m["instance"].(map[string]interface{})

		buf := bytes.NewBuffer(nil)
		err = json.NewEncoder(buf).Encode(m)
		if err != nil {
			panic(err)
		}

		c := packet.DiscoveryCacher{}
		err = json.NewDecoder(buf).Decode(&c)
		if err != nil {
			panic(err)
		}

		d := packet.HardwareTinkerbellV1{
			ID: c.ID,
			Network: packet.Network{
				Interfaces: func() []packet.NetworkInterface {
					ifaces := make([]packet.NetworkInterface, 0, len(c.NetworkPorts))
					pmac := c.PrimaryDataMAC()
					var pip packet.IP
					for _, ip := range c.IPs {
						if ip.Family == 4 && ip.Management {
							pip = ip
							break
						}
					}
					for _, p := range c.NetworkPorts {
						ni := packet.NetworkInterface{
							DHCP: packet.DHCP{
								Arch:      c.Arch,
								IfaceName: p.Name,
								MAC:       p.Data.MAC,
							},
						}
						if *ni.DHCP.MAC == pmac {
							ni.DHCP.IP = pip
							ni.Netboot.AllowPXE = c.AllowPXE
							ni.Netboot.AllowWorkflow = true
						}
						if p.Name == "ipmi0" {
							ni.DHCP.IP = c.IPMI
							family := 4
							if ni.DHCP.IP.Address.To16() != nil {
								family = 6
							}
							ni.DHCP.IP.Family = family
						}
						ifaces = append(ifaces, ni)
					}
					return ifaces
				}(),
			},
			Metadata: packet.Metadata{
				State:        c.State,
				BondingMode:  c.BondingMode,
				Manufacturer: c.Manufacturer,
				Facility: packet.Facility{
					PlanSlug:        c.PlanSlug,
					PlanVersionSlug: c.PlanVersionSlug,
					FacilityCode:    c.FacilityCode,
				},
			},
		}
		d.Metadata.Custom.PreinstalledOS = c.PreinstallOS
		d.Metadata.Custom.PrivateSubnets = c.PrivateSubnets

		// populate metadata.instance straight from cacher version as boot's Instance struct doesn't have all the attributes we care about
		b, err := json.Marshal(d)
		if err != nil {
			panic(err)
		}

		m = nil // "clear" out m so that json.Unmarshal doesn't mix in cacher and tinkerbell data layout in the same `m`
		err = json.Unmarshal(b, &m)
		if err != nil {
			panic(err)
		}
		if m["metadata"] == nil {
			m["metadata"] = map[string]interface{}{}
		}
		metadata := m["metadata"].(map[string]interface{})
		metadata["instance"] = instance
		m["metadata"] = metadata

		err = json.NewEncoder(os.Stdout).Encode(m)
		if err != nil {
			panic(err)
		}
	}
}
