package main

import "strings"

func getConnectedPorts(href string) ([]swtch, error) {
	q := query("include=connected_port.hardware")
	s := struct {
		Ports []struct {
			ConnectedPort struct {
				Hardware swtch
			} `json:"connected_port"`
		}
	}{}
	err := get(&s, href, "/", "ports", q)
	if err != nil {
		return nil, err
	}
	ports := make([]swtch, 0, len(s.Ports))
	for _, port := range s.Ports {
		ports = append(ports, port.ConnectedPort.Hardware)
	}
	return ports, nil
}

func getSwitchesInRack(core bool, rackID string) (map[string]swtch, error) {
	var q string
	if core {
		q = query("name=esr", "type=cluster")
	} else {
		q = query("name=mpr")
	}

	type HRef struct {
		HRef string
	}

	rackhw := struct {
		Hardware []struct {
			hrefID
			Name     string
			Hostname string
		}
	}{}
	err := get(&rackhw, "/server-racks/", rackID, "/hardware", q)
	if err != nil {
		return nil, err
	}

	switches := map[string]swtch{}
	if core {
		for _, rack := range rackhw.Hardware {
			sws, err := getConnectedPorts(rack.HRef)
			if err != nil {
				return nil, err
			}
			for _, sw := range sws {
				if sw.Hostname == "" {
					continue
				}
				ports, err := getSwitchPorts(sw.HRef)
				if err != nil {
					return nil, err
				}
				sw.Ports = ports
				switches[sw.ID] = sw
			}
		}
	} else {
		for _, sw := range rackhw.Hardware {
			switches[sw.ID] = swtch{
				hrefID:   sw.hrefID,
				Hostname: sw.Hostname,
				Name:     sw.Name,
				Ports:    map[string]port{},
			}
		}
	}
	return switches, nil
}

func getSwitchPorts(href string) (map[string]port, error) {
	q := query("include=connected_port.hardware.instance_lite.project_lite.owner", "per_page=200")
	s := struct {
		Ports []struct {
			hrefID
			Name          string
			Type          string
			ConnectedPort struct {
				hrefID
				Name     string
				Type     string
				Hardware struct {
					hrefID
					Hostname string
					Name     string
					State    string
					Type     string
					Instance struct {
						hrefID
						Project struct {
							hrefID
							Owner hrefID
						} `json:"project_lite"`
					} `json:"instance_lite"`
				}
			} `json:"connected_port"`
		}
	}{}
	err := get(&s, href, "/", "ports", q)
	if err != nil {
		return nil, err
	}

	ports := make(map[string]port, len(s.Ports))
	for _, p := range s.Ports {
		switch p.ConnectedPort.Hardware.State {
		case "in_use", "maintenance":
		default:
			continue
		}
		Port := port{
			hrefID: p.hrefID,
			Iface:  p.ConnectedPort.Name,
			Name:   p.Name,
			Type:   p.Type,
			Hardware: struct {
				hrefID
				Hostname string `json:"hostname"`
				Name     string `json:"name"`
				State    string `json:"state"`
				Type     string `json:"type"`
			}{
				hrefID:   p.ConnectedPort.Hardware.hrefID,
				Hostname: p.ConnectedPort.Hardware.Hostname,
				Name:     p.ConnectedPort.Hardware.Name,
				State:    p.ConnectedPort.Hardware.State,
			},
			Instance: p.ConnectedPort.Hardware.Instance.hrefID,
			Project:  p.ConnectedPort.Hardware.Instance.Project.hrefID,
			Owner:    p.ConnectedPort.Hardware.Instance.Project.Owner,
		}
		ports[Port.Name] = Port
	}

	return ports, nil
}

func getSwitchIrbs(hostname string) ([]port, error) {
	t, err := getTable(hostname)
	if err != nil {
		return nil, err
	}

	ndx := strings.Index(hostname, ".")
	subdomain := hostname[ndx+1:]
	ports := make([]port, 0, len(t))
	for _, entry := range t {
		p, ok, err := getIrbPort(subdomain, entry)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		ports = append(ports, p)
	}

	return ports, nil
}
