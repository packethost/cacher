package main

import (
	"context"
	"math/rand"
	"net/url"
	"os"
	"strconv"
	"testing"

	"github.com/packethost/packngo"
	"github.com/packethost/pkg/log"
	"github.com/stretchr/testify/require"
	"gopkg.in/h2non/gock.v1"
)

func TestMain(m *testing.M) {
	os.Setenv("PACKET_ENV", "test")
	os.Setenv("PACKET_VERSION", "0")
	os.Setenv("ROLLBAR_DISABLE", "1")
	os.Setenv("ROLLBAR_TOKEN", "1")

	logger, _ = log.Init("github.com/packethost/cacher")
	setupMetrics("testing")

	os.Exit(m.Run())
}

func TestFetchFacility(t *testing.T) {
	os.Setenv("CACHER_CONCURRENT_FETCHES", "1")
	os.Unsetenv("CACHER_FETCH_PER_PAGE")

	logger = log.Test(t, "github.com/packethost/cacher")
	defer gock.Off()
	facility := "testing" + strconv.Itoa(rand.Int())

	pages := []map[string]interface{}{
		{
			"meta": map[string]interface{}{
				"current_page": 1,
				"last_page":    2,
				"total":        100,
			},
			"Hardware": []map[string]interface{}{
				{
					"id": "1",
				},
			},
		},
		{
			"meta": map[string]interface{}{
				"current_page": 2,
				"last_page":    2,
				"total":        100,
			},
			"Hardware": []map[string]interface{}{
				{
					"id": "2",
				},
			},
		},
	}

	gock.New("https://api.packet.net").
		Get("staff/cacher/hardware").
		MatchParam("facility", facility).
		MatchParam("per_page", "1").
		Reply(200).
		JSON(pages[0])
	for i, m := range pages {
		gock.New("https://api.packet.net").
			Get("staff/cacher/hardware").
			MatchParam("facility", facility).
			MatchParam("page", strconv.Itoa(i+1)).
			Reply(200).
			JSON(m)
	}

	ch := make(chan []map[string]interface{}, len(pages)*10)
	u, err := url.Parse("https://api.packet.net")
	assert := require.New(t)
	assert.NoError(err)

	client := packngo.NewClientWithAuth(os.Getenv("PACKET_CONSUMER_TOKEN"), os.Getenv("PACKET_API_AUTH_TOKEN"), nil)
	err = fetchFacility(context.TODO(), client, u, facility, ch)
	assert.NoError(err)
	assert.Len(ch, len(pages))
	assert.True(gock.IsDone())

	for _, m := range pages {
		p := <-ch
		assert.Equal(m["Hardware"], p)
	}
}
