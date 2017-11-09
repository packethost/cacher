package main

import (
	"fmt"
	"regexp"
)

var (
	// matches irb.$tag, where 3x01 < $tag <= 3x56 | x=[0-3], for type0s
	irbRegexT0 = regexp.MustCompile(`^irb\.(3[0-3](?:0[1-9]|[1-4][0-9]|5[0-6]))$`)

	// matches irb.$tag, where 3x01 < $tag <= 3x48 | x=[0-3], for type1Es
	irbRegexT1E = regexp.MustCompile(`^irb\.(3[0-3](?:0[1-9]|[1-3][0-9]|4[0-8]))$`)

	irbRegex *regexp.Regexp
	getMBC   func(uint) string
	getMB    func(uint) string
	getNode  func(uint) string
)

func getMBCT0(tag uint) string {
	return _getMBC(tag, 56, 3200, 3400)
}

func getMBCT1E(tag uint) string {
	return _getMBC(tag, 48, 3100, 3200)
}

func _getMBC(tag, nodes, mbc1, mbc2 uint) string {
	mbc := 0
	if tag < mbc1 {
		mbc = 1
	} else if tag < mbc2 {
		mbc = 2
	} else {
		panic("invalid tag")
	}
	return fmt.Sprintf("mbc%d", mbc)
}

func _getMB(tag, nodes uint) string {
	mb := ((tag-1-3000)%100)/nodes + 1
	if mb > 24 {
		panic("invalid tag")
	}

	if nodes == 4 && tag/100&1 == 1 {
		mb += 14
	}
	return fmt.Sprintf("mb%d", mb)
}

func getMBT0(tag uint) string {
	return _getMB(tag, 4)
}

func getMBT1E(tag uint) string {
	return _getMB(tag, 2)
}

func _getNode(tag, nodes uint) string {
	return fmt.Sprintf("node%d", (tag%100-1)%nodes+1)
}

func getNodeT0(tag uint) string {
	return _getNode(tag, 4)
}

func getNodeT1E(tag uint) string {
	return _getNode(tag, 2)
}

func getPortFromName(name string) (port, error) {
	q := query("name="+name,
		"exclude=cage,facility,facility_room,instance,manufacturer,notes,plan,problems,rack_spaces,row,server_rack",
		"include=instance_lite.project_lite.owner")
	var s struct {
		Hardware []struct {
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
	}
	err := get(&s, "hardware", q)
	if err != nil {
		return port{}, err
	}
	if len(s.Hardware) != 1 {
		return port{}, nil
	}

	p := s.Hardware[0]
	Port := port{
		hrefID: p.hrefID,
		Iface:  "eth0",
		Type:   "NetworkPort",
		Hardware: struct {
			hrefID
			Hostname string `json:"hostname"`
			Name     string `json:"name"`
			State    string `json:"state"`
			Type     string `json:"type"`
		}{
			hrefID:   p.hrefID,
			Hostname: p.Hostname,
			Name:     p.Name,
			State:    p.State,
		},
		Instance: p.Instance.hrefID,
		Project:  p.Instance.Project.hrefID,
		Owner:    p.Instance.Project.Owner,
	}
	return Port, nil
}

func getIrbPort(subdomain string, entry tableEntry) (port, bool, error) {
	matches := irbRegex.FindStringSubmatch(entry.Name)
	if matches == nil {
		return port{}, false, nil
	}
	var tag uint
	if _, err := fmt.Sscanf(matches[1], "%d", &tag); err != nil {
		return port{}, false, err
	}

	mbc := getMBC(tag)
	mb := getMB(tag)
	node := getNode(tag)

	name := node + "." + mb + "." + mbc + "." + subdomain

	p, err := getPortFromName(name)
	if err != nil {
		return port{}, false, err
	}

	switch p.Hardware.State {
	case "in_use", "maintenance":
	default:
		return port{}, false, nil
	}
	p.Name = entry.Name

	return p, true, nil
}
