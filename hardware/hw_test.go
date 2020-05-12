package hardware

import (
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/require"
)

func TestAdd(t *testing.T) {
	t.Run("errors", func(t *testing.T) {
		assert := require.New(t)
		for _, test := range []string{
			`{}`,
			`{"id": 0}`,
			`{"id": "0"}`,
			`{"id":"5ed553c8-8eab-496a-bbf0-73ce1f900390","instance":{"ip_addresses":[{"address":"localhost"}]}}`,
			`{"id":"5ed553c8-8eab-496a-bbf0-73ce1f900390","ip_addresses":[{"address":"localhost"}]}`,
			`{"id":"5ed553c8-8eab-496a-bbf0-73ce1f900390","network_ports":[{"data":{"mac":"not-a-mac"}}]}`,
		} {
			hw := New()
			id, err := hw.Add(test)
			assert.Error(err)
			assert.Empty(id)
			assert.Empty(hw.byIP)
			assert.Empty(hw.byMAC)
		}
	})

	t.Run("ips", func(t *testing.T) {
		assert := require.New(t)
		for _, test := range []struct {
			id string
			j  string
			n  int
		}{
			{id: uuid.New().String(), j: `{"id":"%s","instance":{"ip_addresses":[{"address":"192.168.0.1"}]}}`, n: 1},
			{id: uuid.New().String(), j: `{"id":"%s","instance":{"ip_addresses":[{"address":"192.168.0.1"},{"address":"192.168.0.2"}]}}`, n: 2},
			{id: uuid.New().String(), j: `{"id":"%s","instance":{"ip_addresses":[{"address":"192.168.0.1"},{"fubar":true}]}}`, n: 1},
			{id: uuid.New().String(), j: `{"id":"%s","ip_addresses":[{"address":"192.168.0.1"}]}`, n: 1},
			{id: uuid.New().String(), j: `{"id":"%s","ip_addresses":[{"address":"192.168.0.1"},{"address":"192.168.0.2"}]}`, n: 2},
			{id: uuid.New().String(), j: `{"id":"%s","ip_addresses":[{"address":"192.168.0.1"},{"fubar":true}]}`, n: 1},
			{id: uuid.New().String(), j: `{"id":"%s","instance":{"ip_addresses":[{"address":"192.168.0.1"},{"address":"192.168.0.2"}]},"ip_addresses":[{"address":"192.168.1.1"},{"address":"192.168.1.2"}]}`, n: 4},
		} {
			hw := New()
			id, err := hw.Add(fmt.Sprintf(test.j, test.id))
			assert.NoError(err)
			assert.Equal(test.id, id)
			assert.Len(hw.hw, 1)
			assert.Len(hw.byIP, test.n)
		}
	})

	t.Run("macs", func(t *testing.T) {
		assert := require.New(t)
		for _, test := range []struct {
			id string
			j  string
			n  int
		}{
			{id: uuid.New().String(), j: `{"id":"%s","network_ports":[{"data":{"mac":"00:00:00:00:00:01"}}]}`, n: 1},
			{id: uuid.New().String(), j: `{"id":"%s","network_ports":[{"data":{"mac":"00:00:00:00:0a:01"}}]}`, n: 1},
			{id: uuid.New().String(), j: `{"id":"%s","network_ports":[{"data":{"mac":"00:00:00:00:0A:01"}}]}`, n: 1},
			{id: uuid.New().String(), j: `{"id":"%s","network_ports":[{"data":{"mac":"00:00:00:00:0A:01"}},{"data":{"mac":"00:00:00:00:00:02"}}]}`, n: 2},
			{id: uuid.New().String(), j: `{"id":"%s","network_ports":[{"data":{"mac":"00:00:00:00:0A:01"}},{"data":{"mac":""}}]}`, n: 1},
			{id: uuid.New().String(), j: `{"id":"%s","network_ports":[{"data":{"mac":"00:00:00:00:0A:01"}},{"data":{"mac":null}}]}`, n: 1},
		} {
			hw := New()
			id, err := hw.Add(fmt.Sprintf(test.j, test.id))
			assert.NoError(err)
			assert.Equal(test.id, id)
			assert.Len(hw.hw, 1)
			assert.Len(hw.byMAC, test.n)
		}
	})

	t.Run("ip changes", func(t *testing.T) {
		assert := require.New(t)
		hw := New()

		// same id gets some ips changed
		_, err := hw.Add(`{"id":"5ed553c8-8eab-496a-bbf0-73ce1f900390","instance":{"ip_addresses":[{"address":"10.0.0.1"}, {"address":"10.0.0.2"}]}, "ip_addresses":[{"address":"10.0.1.1"}, {"address":"10.0.1.2"}]}`)
		assert.NoError(err)
		_, err = hw.Add(`{"id":"5ed553c8-8eab-496a-bbf0-73ce1f900390","instance":{"ip_addresses":[{"address":"10.0.0.1"}, {"address":"10.0.0.3"}]}, "ip_addresses":[{"address":"10.0.1.1"}, {"address":"10.0.1.3"}]}`)
		assert.NoError(err)
		assert.Len(hw.hw, 1)
		assert.Len(hw.byIP, 4)

		// new id has ip of old one, will override indexes
		_, err = hw.Add(`{"id":"5ed553c8-8eab-496a-bbf0-73ce1f900390","ip_addresses":[{"address":"10.0.0.1"},{"address":"10.0.0.2"}]}`)
		assert.NoError(err)
		_, err = hw.Add(`{"id":"5ed553c8-8eab-496a-bbf0-73ce1f900391","ip_addresses":[{"address":"10.0.0.2"},{"address":"10.0.0.3"}]}`)
		assert.NoError(err)

		assert.Len(hw.hw, 2)
		assert.Len(hw.byIP, 3)

		// this id had only 1 ip pointing back at it, now its gone.
		// so the byIP cleanup should skip the ip from the entry since its not pointing at this entry and we are left with only 2 ip
		_, err = hw.Add(`{"id":"5ed553c8-8eab-496a-bbf0-73ce1f900390"}`)
		assert.NoError(err)
		assert.Len(hw.hw, 2)
		assert.Len(hw.byIP, 2)
	})
	t.Run("mac changes", func(t *testing.T) {
		// TODO belongs in test of ByMAC or own top-level since it tests both Add and ByMAC paths
		assert := require.New(t)
		hw := New()

		// setup
		_, err := hw.Add(`{"id":"5ed553c8-8eab-496a-bbf0-73ce1f900390","network_ports":[{"data":{"mac":"00:00:00:00:00:01"}}]}`)
		assert.NoError(err)
		_, err = hw.Add(`{"id":"5ed553c8-8eab-496a-bbf0-73ce1f900390","network_ports":[{"data":{"mac":"00:00:00:00:00:02"}}]}`)
		assert.NoError(err)

		// new entry has ip of old one, will override indexes
		_, err = hw.Add(`{"id":"5ed553c8-8eab-496a-bbf0-73ce1f900390","network_ports":[{"data":{"mac":"00:00:00:00:00:01"}},{"data":{"mac":"00:00:00:00:00:02"}}]}`)
		assert.NoError(err)
		_, err = hw.Add(`{"id":"5ed553c8-8eab-496a-bbf0-73ce1f900391","network_ports":[{"data":{"mac":"00:00:00:00:00:02"}},{"data":{"mac":"00:00:00:00:00:03"}}]}`)
		assert.NoError(err)

		assert.Len(hw.hw, 2)
		assert.Len(hw.byMAC, 3)
		// this entry had only 1 ip pointing back at it, now its gone, so the byIP cleanup should skip the ip from the entry since its not pointing at this entry
		_, err = hw.Add(`{"id":"5ed553c8-8eab-496a-bbf0-73ce1f900390"}`)
		assert.NoError(err)
		assert.Len(hw.hw, 2)
		assert.Len(hw.byMAC, 2)
	})

	t.Run("deletion", func(t *testing.T) {
		assert := require.New(t)
		hw := New()

		_, err := hw.Add(`{"id":"5ed553c8-8eab-496a-bbf0-73ce1f900390","network_ports":[{"data":{"mac":"00:00:00:00:00:01"}}]}`)
		assert.NoError(err)
		_, err = hw.Add(`{"id":"5ed553c8-8eab-496a-bbf0-73ce1f900391","network_ports":[{"data":{"mac":"00:00:00:00:00:0a"}}]}`)
		assert.NoError(err)
		assert.Len(hw.hw, 2)
		assert.Len(hw.byMAC, 2)
		_, err = hw.Add(`{"id":"5ed553c8-8eab-496a-bbf0-73ce1f900390","state":"deleted"}`)
		assert.NoError(err)
		assert.Len(hw.hw, 1)
		assert.Len(hw.byMAC, 1)
	})
}

func TestGauge(t *testing.T) {
	assert := require.New(t)

	g := prometheus.NewGauge(prometheus.GaugeOpts{})
	assert.Equal(0, int(testutil.ToFloat64(g)))

	hw := New(Gauge(g))
	id, _ := hw.Add(fmt.Sprintf(`{"id":"%s"}`, uuid.New().String()))
	assert.Equal(1, int(testutil.ToFloat64(g)))
	hw.Add(fmt.Sprintf(`{"id":"%s"}`, uuid.New().String()))
	assert.Equal(2, int(testutil.ToFloat64(g)))
	hw.Add(fmt.Sprintf(`{"id":"%s"}`, id))
	assert.Equal(2, int(testutil.ToFloat64(g)))
	hw.Add(fmt.Sprintf(`{"id":"%s", "state":"deleted"}`, id))
	assert.Equal(1, int(testutil.ToFloat64(g)))

}

func TestAll(t *testing.T) {
	assert := require.New(t)

	jsons := map[string]bool{
		`{"id": "` + uuid.New().String() + `"}`: false,
		`{"id": "` + uuid.New().String() + `"}`: false,
		`{"id": "` + uuid.New().String() + `"}`: false,
	}
	l := len(jsons)

	hw := New()
	for j := range jsons {
		id, err := hw.Add(j)
		assert.NoError(err)
		assert.NotEmpty(id)
	}
	assert.Len(hw.hw, l)

	assert.NoError(hw.All(func(j string) error {
		jsons[j] = true
		return nil
	}))

	// ensure all were found, and no new ones spontaneously popped into existence
	assert.Len(jsons, l)
	for j, found := range jsons {
		assert.True(found, j)
	}

	errstr := "some error"
	assert.EqualError(hw.All(func(j string) error {
		return fmt.Errorf(errstr)
	}), "callback function returned an error: "+errstr)
}

func TestByID(t *testing.T) {
	assert := require.New(t)

	id := uuid.New().String()
	j := `{"id":"` + id + `","some-other-random-id":"` + uuid.New().String() + `"}`

	hw := New()
	hw.Add(j)
	h, err := hw.ByID(id)
	assert.NoError(err)
	assert.Equal(j, h)
}

func TestByIP(t *testing.T) {
	assert := require.New(t)

	for _, test := range []struct {
		id    string
		ip    string
		j     string
		empty bool
		err   string
	}{
		{id: uuid.New().String(), ip: "192.168.1.1", j: `{"id":"%s","ip_addresses":[{"address":"192.168.1.1"}]}`},
		{id: uuid.New().String(), ip: "192.168.1.2", j: `{"id":"%s","instance":{"ip_addresses":[{"address":"192.168.1.2"}]}}`},
		{id: uuid.New().String(), ip: "192.168.1.3", j: `{"id":"%s","instance":{"ip_addresses":[{"address":"192.168.1.33"}]}}`, empty: true},
		{id: uuid.New().String(), ip: "localhost", j: `{"id":"%s","instance":{"ip_addresses":[{"address":"192.168.1.1"}]}}`, err: "failed to parse ip"},
		{id: uuid.New().String(), ip: "::c0c0", j: `{"id":"%s","instance":{"ip_addresses":[{"address":"::c0c0"}]}}`},
		{id: uuid.New().String(), ip: "::c0c0", j: `{"id":"%s","instance":{"ip_addresses":[{"address":"::C0C0"}]}}`},
	} {
		t.Log("ip:", test.ip)
		j := fmt.Sprintf(test.j, test.id)
		hw := New()
		hw.Add(j)
		if test.empty {
			j = ""
		}
		h, err := hw.ByIP(test.ip)
		if test.err == "" {
			assert.NoError(err)
			assert.Equal(j, h)
		} else {
			assert.EqualError(err, test.err)
			assert.Empty(h)
		}
	}

	id1 := uuid.New().String()
	j1 := fmt.Sprintf(`{"id":"%s","instance":{"ip_addresses":[{"address":"10.0.0.1"}, {"address":"10.0.0.2"}]}, "ip_addresses":[{"address":"10.0.0.3"}, {"address":"10.0.0.4"}]}`, id1)
	id2 := uuid.New().String()
	j2 := fmt.Sprintf(`{"id":"%s","instance":{"ip_addresses":[{"address":"10.0.0.5"}, {"address":"10.0.0.6"}]}, "ip_addresses":[{"address":"10.0.0.7"}, {"address":"10.0.0.8"}]}`, id2)

	hw := New()
	_, err := hw.Add(j1)
	assert.NoError(err)
	_, err = hw.Add(j2)
	assert.NoError(err)

	for _, ip := range []string{"1", "2", "3", "4"} {
		j, err := hw.ByIP("10.0.0." + ip)
		assert.NoError(err)
		assert.Equal(j1, j)
	}
	for _, ip := range []string{"5", "6", "7", "8"} {
		j, err := hw.ByIP("10.0.0." + ip)
		assert.NoError(err)
		assert.Equal(j2, j)
	}

	j1 = fmt.Sprintf(`{"id":"%s","instance":{"ip_addresses":[{"address":"10.0.0.1"}, {"address":"10.0.0.6"}]}, "ip_addresses":[{"address":"10.0.0.3"}, {"address":"10.0.0.8"}]}`, id1)
	_, err = hw.Add(j1)
	assert.NoError(err)
	// j1's new ips
	for _, ip := range []string{"1", "6", "3", "8"} {
		j, err := hw.ByIP("10.0.0." + ip)
		assert.NoError(err)
		assert.Equal(j1, j)
	}
	// j1's old ips that went away and should be missing
	for _, ip := range []string{"2", "4"} {
		j, err := hw.ByIP("10.0.0." + ip)
		assert.NoError(err)
		assert.Empty(j)
	}
	// j2 only has 2 ips now
	for _, ip := range []string{"5", "7"} {
		j, err := hw.ByIP("10.0.0." + ip)
		assert.NoError(err)
		assert.Equal(j2, j)
	}
}

func TestByMAC(t *testing.T) {
	assert := require.New(t)

	for _, test := range []struct {
		id    string
		mac   string
		j     string
		empty bool
		err   string
	}{
		{id: uuid.New().String(), mac: "00:00:00:00:00:01", j: `{"id":"%s","network_ports":[{"data":{"mac":"00:00:00:00:00:01"}}]}`},
		{id: uuid.New().String(), mac: "00:00:00:00:00:0a", j: `{"id":"%s","network_ports":[{"data":{"mac":"00:00:00:00:00:0a"}}]}`},
		{id: uuid.New().String(), mac: "00:00:00:00:00:0A", j: `{"id":"%s","network_ports":[{"data":{"mac":"00:00:00:00:00:0A"}}]}`},
		{id: uuid.New().String(), mac: "00-00-00-00-00-0b", j: `{"id":"%s","network_ports":[{"data":{"mac":"00:00:00:00:00:0b"}}]}`},
		{id: uuid.New().String(), mac: "00:00:00:00:00:0c", j: `{"id":"%s","network_ports":[{"data":{"mac":"00:00:00:00:00:cc"}}]}`, empty: true},
		{id: uuid.New().String(), mac: "not-a-mac", j: `{"id":"%s","network_ports":[{"data":{"mac":"00:00:00:00:00:bb"}}]}`, err: "failed to parse mac: address not-a-mac: invalid MAC address"},
	} {
		t.Log("mac:", test.mac)
		j := fmt.Sprintf(test.j, test.id)
		hw := New()
		hw.Add(j)
		if test.empty {
			j = ""
		}
		h, err := hw.ByMAC(test.mac)
		if test.err == "" {
			assert.NoError(err)
			assert.Equal(j, h)
		} else {
			assert.EqualError(err, test.err)
			assert.Empty(h)
		}
	}

	id1 := uuid.New().String()
	j1 := fmt.Sprintf(`{"id":"%s","network_ports":[{"data":{"mac":"00:00:00:00:00:01"}},{"data":{"mac":"00:00:00:00:00:02"}}]}`, id1)
	id2 := uuid.New().String()
	j2 := fmt.Sprintf(`{"id":"%s","network_ports":[{"data":{"mac":"00:00:00:00:00:03"}},{"data":{"mac":"00:00:00:00:00:04"}}]}`, id2)

	hw := New()
	_, err := hw.Add(j1)
	assert.NoError(err)
	_, err = hw.Add(j2)
	assert.NoError(err)

	for _, mac := range []string{"01", "02"} {
		j, err := hw.ByMAC("00:00:00:00:00:" + mac)
		assert.NoError(err)
		assert.Equal(j1, j)
	}
	for _, mac := range []string{"03", "04"} {
		j, err := hw.ByMAC("00:00:00:00:00:" + mac)
		assert.NoError(err)
		assert.Equal(j2, j)
	}

	j1 = fmt.Sprintf(`{"id":"%s","network_ports":[{"data":{"mac":"00:00:00:00:00:01"}},{"data":{"mac":"00:00:00:00:00:04"}}]}`, id1)
	_, err = hw.Add(j1)
	assert.NoError(err)
	// j1's new macs
	for _, mac := range []string{"01", "04"} {
		j, err := hw.ByMAC("00:00:00:00:00:" + mac)
		assert.NoError(err)
		assert.Equal(j1, j)
	}
	// j1's old mac that went away and should be missing
	for _, mac := range []string{"02"} {
		j, err := hw.ByMAC("00:00:00:00:00:" + mac)
		assert.NoError(err)
		assert.Empty(j)
	}
	// j2 only has 1 mac now
	for _, mac := range []string{"03"} {
		j, err := hw.ByMAC("00:00:00:00:00:" + mac)
		assert.NoError(err)
		assert.Equal(j2, j)
	}
}
