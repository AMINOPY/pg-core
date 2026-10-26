package main

import "testing"

func TestLocalSOCKSAddressRequiresLoopbackNoAuth(t *testing.T) {
	good := []byte(`{"inbounds":[{"listen":"127.0.0.1","port":1080,"protocol":"socks","settings":{"auth":"noauth"}}]}`)
	got, err := localSOCKSAddress(good)
	if err != nil || got != "127.0.0.1:1080" {
		t.Fatalf("address=%q error=%v", got, err)
	}
	badProfiles := [][]byte{
		[]byte(`{"inbounds":[{"listen":"0.0.0.0","port":1080,"protocol":"socks","settings":{"auth":"noauth"}}]}`),
		[]byte(`{"inbounds":[{"listen":"127.0.0.1","port":1080,"protocol":"socks","settings":{"auth":"password","accounts":[{"user":"u","pass":"p"}]}}]}`),
		[]byte(`{"inbounds":[{"listen":"127.0.0.1","port":70000,"protocol":"socks","settings":{"auth":"noauth"}}]}`),
		[]byte(`{"inbounds":[{"listen":"127.0.0.1","port":1080,"protocol":"http"}]}`),
		[]byte(`not json`),
	}
	for _, profile := range badProfiles {
		if got, err := localSOCKSAddress(profile); err == nil {
			t.Errorf("unsafe/invalid profile unexpectedly accepted with address %q: %s", got, profile)
		}
	}
}
