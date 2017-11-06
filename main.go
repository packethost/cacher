package main

import (
	"errors"
	"fmt"
	"os"
	"sync"
)

func newFacility(code string) (*facility, error) {
	v := map[string][]struct {
		Code string
		HRef string
		ID   string
		Name string
	}{}
	err := get(&v, "facilities")
	if err != nil {
		return nil, err
	}

	facs, ok := v["facilities"]
	if !ok {
		return nil, errors.New("error fetching facilities")
	}

	i := 0
	found := false
	for i = range facs {
		f := facs[i]
		if f.Code == code {
			found = true
			break
		}
	}
	if !found {
		return nil, errors.New("facility is not available")
	}

	f := facs[i]
	return &facility{
		hrefID: hrefID{HRef: f.HRef, ID: f.ID},
		Code:   f.Code,
	}, nil
}

func (f *facility) getRacks() error {
	v := struct {
		ID            string
		Code          string
		FacilityRooms []struct {
			Cages []struct {
				Rows []struct {
					Racks []rack `json:"server_racks"`
				} `json:"rows"`
			} `json:"cages"`
		} `json:"facility_rooms"`
	}{}

	err := get(&v, "facilities/"+f.ID, query("include=facility_rooms.cages.rows.server_racks"))
	if err != nil {
		return err
	}

	f.Racks = map[string]rack{}
	for _, room := range v.FacilityRooms {
		for _, cage := range room.Cages {
			for _, row := range cage.Rows {
				for _, rack := range row.Racks {
					f.Racks[rack.ID] = rack
				}
			}
		}
	}
	return nil
}

func (f *facility) getRackSwitches() error {
	for _, rack := range f.Racks {
		core := false
		switch f.Code {
		case "ams1", "ewr1", "nrt1", "sjc1":
			core = true
		}
		switches, err := getSwitchesInRack(core, rack.ID)
		if err != nil {
			return fmt.Errorf(`msg="failed to get rack switches", rack.id="%s", rack.name="%s" error="%s"\n`, rack.ID, rack.Name, err.Error())
		}
		rack.Switches = switches
		f.Racks[rack.ID] = rack
	}
	return nil
}

func (f *facility) getIrbs() error {
	type result struct {
		hostname string
		ports    []port
		err      error
	}

	switchPorts := make(map[string][]port)
	results := make(chan result, len(f.Racks))
	wg := sync.WaitGroup{}
	for _, rack := range f.Racks {
		for _, swtch := range rack.Switches {
			if _, ok := switchPorts[swtch.Hostname]; ok {
				continue
			}
			switchPorts[swtch.Hostname] = nil

			wg.Add(1)
			go func(hostname string) {
				ports, err := getSwitchIrbs(hostname)
				results <- result{
					hostname: hostname,
					ports:    ports,
					err:      err,
				}
				wg.Done()
			}(swtch.Hostname)
		}
	}
	go func() {
		wg.Wait()
		close(results)
	}()

	for r := range results {
		if r.err != nil {
			return fmt.Errorf("failed to get irbs, hostname=%s, error=%v", r.hostname, r.err)
		}
		switchPorts[r.hostname] = r.ports
	}

	for _, rack := range f.Racks {
		for _, swtch := range rack.Switches {
			for _, port := range switchPorts[swtch.Hostname] {
				swtch.Ports[port.Name] = port
			}
			rack.Switches[swtch.ID] = swtch
		}
	}
	return nil
}

func main() {
	facility, err := newFacility(os.Getenv("PACKET_ENV"))
	if err != nil {
		panic(err)
	}
	switch facility.Code {
	case "ams1", "ewr1", "nrt1", "sjc1":
		getMB = getMBT0
		getMBC = getMBCT0
		getNode = getNodeT0
		irbRegex = irbRegexT0
	default:
		irbRegex = irbRegexT1E
		getMB = getMBT1E
		getMBC = getMBCT1E
		getNode = getNodeT1E
	}

	if err = connectCache(); err != nil {
		panic(err)
	}
	if err = facility.getRacks(); err != nil {
		panic(err)
	}
	if err = facility.getRackSwitches(); err != nil {
		panic(err)
	}
	if err = facility.getIrbs(); err != nil {
		panic(err)
	}
	if err = setCache(facility); err != nil {
		panic(err)
	}
}
