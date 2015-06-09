package rstatd

import (
	"strconv"
	"testing"
	"time"
)

func TestRstatdPort(t *testing.T) {
	port, err := rstatdPort()
	if err != nil {
		t.Fatal(err)
	}
	if port == 0 {
		t.Errorf("expected port to be non-zero")
	}
}

func TestFetch(t *testing.T) {
	cli := new(Client)
	port, err := rstatdPort()
	if err != nil {
		t.Fatal(err)
	}
	res, err := cli.readStats("0.0.0.0:" + strconv.FormatUint(uint64(port), 10))
	if err != nil {
		t.Errorf("failed to fetch stats %v", err)
		return
	}
	if len(res) < 116 {
		t.Errorf("short response length %d", len(res))
	}
}

func TestReadStatsWithClient(t *testing.T) {
	cli := new(Client)
	start := time.Now().Truncate(time.Second)
	stats, err := cli.ReadStats()
	if err != nil {
		t.Error(err)
	}
	end := time.Now()
	if stats.CurrTime.Before(start) {
		t.Errorf("curr time %s is before start %s", stats.CurrTime, start)
	}
	if stats.CurrTime.After(end) {
		t.Errorf("curr time %s is after end %s", stats.CurrTime, end)
	}
}

func TestReadStats(t *testing.T) {
	start := time.Now().Truncate(time.Second)
	stats, err := ReadStats()
	if err != nil {
		t.Error(err)
	}
	end := time.Now()
	if stats.CurrTime.Before(start) {
		t.Errorf("curr time %s is before start %s", stats.CurrTime, start)
	}
	if stats.CurrTime.After(end) {
		t.Errorf("curr time %s is after end %s", stats.CurrTime, end)
	}
}
