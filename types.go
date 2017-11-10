package main

type hrefID struct {
	HRef string `json:"href"`
	ID   string `json:"id"`
}

type port struct {
	hrefID
	Iface    string `json:"iface"`
	Name     string `json:"name"`
	Index    uint   `json:"index"`
	Type     string `json:"type"`
	Hardware struct {
		hrefID
		Hostname string `json:"hostname"`
		Name     string `json:"name"`
		State    string `json:"state"`
		Type     string `json:"type"`
	}
	Instance hrefID `json:"instance"`
	Owner    hrefID `json:"owner"`
	Project  hrefID `json:"project"`
}

type swtch struct {
	hrefID
	Hostname string          `json:"hostname"`
	Name     string          `json:"name"`
	Type     string          `json:"type"`
	Ports    map[string]port `json:"ports"`
}

type rack struct {
	hrefID
	Name     string           `json:"name"`
	Hostname string           `json:"hostname"`
	Switches map[string]swtch `json:"switches"`
}

type facility struct {
	hrefID
	Code  string          `json:"code"`
	Racks map[string]rack `json:"racks"`
}
