package main

import "testing"

func TestGetMBT0(t *testing.T) {
	tests := []struct {
		tag  uint
		want string
	}{
		{3001, "mb1"},
		{3004, "mb1"},
		{3005, "mb2"},
		{3008, "mb2"},
		{3049, "mb13"},
		{3052, "mb13"},
		{3053, "mb14"},
		{3056, "mb14"},

		{3101, "mb15"},
		{3104, "mb15"},
		{3105, "mb16"},
		{3108, "mb16"},
		{3149, "mb27"},
		{3152, "mb27"},
		{3153, "mb28"},
		{3156, "mb28"},

		{3201, "mb1"},
		{3204, "mb1"},
		{3205, "mb2"},
		{3208, "mb2"},
		{3249, "mb13"},
		{3252, "mb13"},
		{3253, "mb14"},
		{3256, "mb14"},

		{3301, "mb15"},
		{3304, "mb15"},
		{3305, "mb16"},
		{3308, "mb16"},
		{3349, "mb27"},
		{3352, "mb27"},
		{3353, "mb28"},
		{3356, "mb28"},
	}

	for _, test := range tests {
		got := getMBT0(test.tag)
		if got != test.want {
			t.Errorf("tag: %d, want: %s, got: %s", test.tag, test.want, got)
		}
	}
}

func TestGetMBT1E(t *testing.T) {
	tests := []struct {
		tag  uint
		want string
	}{
		{3001, "mb1"},
		{3002, "mb1"},
		{3003, "mb2"},
		{3004, "mb2"},
		{3025, "mb13"},
		{3026, "mb13"},
		{3027, "mb14"},
		{3028, "mb14"},

		{3101, "mb1"},
		{3102, "mb1"},
		{3103, "mb2"},
		{3104, "mb2"},
		{3125, "mb13"},
		{3126, "mb13"},
		{3127, "mb14"},
		{3128, "mb14"},
	}

	for _, test := range tests {
		got := getMBT1E(test.tag)
		if got != test.want {
			t.Errorf("tag: %d, want: %s, got: %s", test.tag, test.want, got)
		}
	}
}

func TestGetMBCT0(t *testing.T) {
	tests := []struct {
		tag  uint
		want string
	}{
		{3001, "mbc1"},
		{3056, "mbc1"},
		{3101, "mbc1"},
		{3156, "mbc1"},

		{3201, "mbc2"},
		{3256, "mbc2"},
		{3301, "mbc2"},
		{3356, "mbc2"},
	}

	for _, test := range tests {
		got := getMBCT0(test.tag)
		if got != test.want {
			t.Errorf("tag: %d, want: %s, got: %s", test.tag, test.want, got)
		}
	}
}

func TestGetMBCT1E(t *testing.T) {
	tests := []struct {
		tag  uint
		want string
	}{
		{3001, "mbc1"},
		{3048, "mbc1"},

		{3101, "mbc2"},
		{3148, "mbc2"},
	}

	for _, test := range tests {
		got := getMBCT1E(test.tag)
		if got != test.want {
			t.Errorf("tag: %d, want: %s, got: %s", test.tag, test.want, got)
		}
	}
}

func TestGetNodeT0(t *testing.T) {
	tests := []struct {
		tag  uint
		want string
	}{
		{3001, "node1"},
		{3002, "node2"},
		{3003, "node3"},
		{3004, "node4"},
		{3053, "node1"},
		{3054, "node2"},
		{3055, "node3"},
		{3056, "node4"},

		{3101, "node1"},
		{3102, "node2"},
		{3103, "node3"},
		{3104, "node4"},
		{3153, "node1"},
		{3154, "node2"},
		{3155, "node3"},
		{3156, "node4"},

		{3201, "node1"},
		{3202, "node2"},
		{3203, "node3"},
		{3204, "node4"},
		{3253, "node1"},
		{3254, "node2"},
		{3255, "node3"},
		{3256, "node4"},

		{3301, "node1"},
		{3302, "node2"},
		{3303, "node3"},
		{3304, "node4"},
		{3353, "node1"},
		{3354, "node2"},
		{3355, "node3"},
		{3356, "node4"},
	}

	for _, test := range tests {
		got := getNodeT0(test.tag)
		if got != test.want {
			t.Errorf("tag: %d, want: %s, got: %s", test.tag, test.want, got)
		}
	}
}

func TestGetNodeT1E(t *testing.T) {
	tests := []struct {
		tag  uint
		want string
	}{
		{3001, "node1"},
		{3002, "node2"},
		{3003, "node1"},
		{3004, "node2"},
		{3045, "node1"},
		{3046, "node2"},
		{3047, "node1"},
		{3048, "node2"},

		{3101, "node1"},
		{3102, "node2"},
		{3103, "node1"},
		{3104, "node2"},
		{3145, "node1"},
		{3146, "node2"},
		{3147, "node1"},
		{3148, "node2"},
	}

	for _, test := range tests {
		got := getNodeT1E(test.tag)
		if got != test.want {
			t.Errorf("tag: %d, want: %s, got: %s", test.tag, test.want, got)
		}
	}
}
