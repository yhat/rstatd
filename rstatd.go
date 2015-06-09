/*
Package rstatd provides a rstatd client implementation.

rstatd is a monitoring tool for the linux kernel.

To query a local instance of statd, use the ReadStats function.

   stats, err := rstatd.ReadStats()
   if err != nil {
       // handle error
   }
   fmt.Println(stats.CPUUser, stats.CPUNice, stats.CPUSys, stats.CPUIdle)

For remote instances, construct a client before making the call.

   cli := rstatd.Client{
       Host: "10.0.0.1",
       Port: "792",
   }
   stats, err := cli.ReadStats()

If the port is left empty, the client will request the daemon's port from
rpcbind, which is assumed to be accessable at port 111.
*/
package rstatd

import (
	"encoding/binary"
	"fmt"
	"math/rand"
	"net"
	"strconv"
	"strings"
	"time"
)

var localhostClient = &Client{}

// ReadStats returns stats from localhost.
func ReadStats() (*Stats, error) { return localhostClient.ReadStats() }

type Client struct {
	// The hostname of the rstatd server.
	// If empty, '0.0.0.0' is implied.
	Host string
	// The port of the statd daemon.
	// If empty, the client will request the daemon's port from the
	// rpcbind process at port 111.
	Port string
}

type Stats struct {
	CPUUser uint32
	CPUNice uint32
	CPUSys  uint32
	CPUIdle uint32

	DiskTransfers [4]uint32

	PagesIn  uint32
	PagesOut uint32

	PageSwapsIn  uint32
	PageSwapsOut uint32

	Interrupts      uint32
	ContextSwitches uint32

	NetIPackets   uint32
	NetIErrors    uint32
	NetOPackets   uint32
	NetOErrors    uint32
	NetCollisions uint32

	AverageRunQueryLen [3]uint32

	BootTime time.Time
	CurrTime time.Time
}

// struct statstime {				/* RSTATVERS_TIME */
// 	int cp_time[CPUSTATES];
// 	int dk_xfer[DK_NDRIVE];
// 	unsigned int v_pgpgin;	/* these are cumulative sum */
// 	unsigned int v_pgpgout;
// 	unsigned int v_pswpin;
// 	unsigned int v_pswpout;
// 	unsigned int v_intr;
// 	int if_ipackets;
// 	int if_ierrors;
// 	int if_oerrors;
// 	int if_collisions;
// 	unsigned int v_swtch;
// 	int avenrun[3];         /* scaled by FSCALE */
// 	rstat_timeval boottime;
// 	rstat_timeval curtime;
// 	int if_opackets;
// };

// ReadStats reads the stats from the machine.
// If the port of the client is not specified.
func (c *Client) ReadStats() (*Stats, error) {
	s := new(Stats)
	port := strings.TrimLeft(c.Port, ":")
	if port == "" {
		p, err := rstatdPort()
		if err != nil {
			return nil, err
		}
		port = strconv.FormatUint(uint64(p), 10)
	}
	rawResp, err := c.readStats(c.Host + ":" + port)
	if err != nil {
		return nil, err
	}
	n := len(rawResp)
	if n < 116 {
		return nil, fmt.Errorf("rstatd: bad response length from daemon. expected at least 116 bytes, got %d", n)
	}

	// the first 12 bytes of the response aren't relavent
	rawResp = rawResp[12:]

	next := func() uint32 {
		v := binary.BigEndian.Uint32(rawResp[:4])
		rawResp = rawResp[4:]
		return v
	}

	s.CPUUser, s.CPUNice, s.CPUSys, s.CPUIdle = next(), next(), next(), next()
	for i := 0; i < 4; i++ {
		s.DiskTransfers[i] = next()
	}
	s.PagesIn, s.PagesOut = next(), next()
	s.PageSwapsIn, s.PageSwapsOut = next(), next()
	s.Interrupts = next()
	s.NetIPackets, s.NetIErrors = next(), next()
	s.NetOErrors, s.NetCollisions = next(), next()
	s.ContextSwitches = next()
	for i := 0; i < 3; i++ {
		s.AverageRunQueryLen[i] = next() / 256
	}

	s.BootTime = time.Unix(int64(next()), int64(next()))
	s.CurrTime = time.Unix(int64(next()), int64(next()))
	s.NetOPackets = next()
	return s, nil
}

// stack encodes a set of uint32 values in big endian order as a byte slice.
func stack(words ...uint32) []byte {
	b := make([]byte, len(words)*4)
	wordBuff := make([]byte, 4)

	for i, w := range words {
		binary.BigEndian.PutUint32(wordBuff, w)
		offset := i * 4
		for j := 0; j < 4; j++ {
			b[offset+j] = wordBuff[j]
		}
	}
	return b
}

func (c *Client) readStats(addr string) ([]byte, error) {
	conn, err := net.Dial("udp", addr)
	if err != nil {
		return nil, fmt.Errorf("rstatd: failed to connect to daemon %v", err)
	}
	defer conn.Close()
	tId := rand.Uint32()
	req := stack(
		tId,        // transaction id
		0x00000000, // request type (CALL)
		0x00000002, // rpc version
		0x000186a1, // program (rstat)
		0x00000003, // version
		0x00000001, // procedure
		0x00000000,
		0x00000000,
		0x00000000,
		0x00000000,
	)
	resp, err := doRPCTrans(conn, req, tId)
	if err != nil {
		return nil, fmt.Errorf("rstatd: daemon request failed: %v", err)
	}
	return resp, nil
}

// doRPCTrans performs an RPC transaction and validates the response.
func doRPCTrans(conn net.Conn, req []byte, transId uint32) ([]byte, error) {
	if _, err := conn.Write(req); err != nil {
		return nil, fmt.Errorf("failed to write request %v", err)
	}
	resp := make([]byte, 2048)
	n, err := conn.Read(resp)
	if err != nil {
		return nil, fmt.Errorf("failed to read response %v", err)
	}
	if n < 12 {
		return nil, fmt.Errorf("invalid response length %d", n)
	}
	resp = resp[:n]

	next := func() uint32 {
		v := binary.BigEndian.Uint32(resp[:4])
		resp = resp[4:]
		return v
	}
	if next() != transId {
		return nil, fmt.Errorf("transcation id mismatch from rpc request")
	}
	if next() != 0x01 {
		return nil, fmt.Errorf("invalid response from rpc request")
	}
	if next() != 0x00 {
		return nil, fmt.Errorf("rpc request failed")
	}
	return resp, nil
}

// rstatdPort asks the local rpcbind process what port the rstatd process
// is listening on
func rstatdPort() (uint32, error) {
	conn, err := net.Dial("udp", "0.0.0.0:111")
	if err != nil {
		return 0, fmt.Errorf("rstatd: failed to dial rpcbind service %v", err)
	}
	defer conn.Close()
	tId := rand.Uint32()
	req := stack(
		tId,        // transaction id
		0x00000000, // request type (CALL)
		0x00000002, // rpc version
		0x000186a0, // program (portmap)
		0x00000002, // version
		0x00000003, // procedure
		0x00000000,
		0x00000000,
		0x00000000,
		0x00000000,
		0x000186a1, // program to look up (rstat)
		0x00000003, // version
		0x00000011, // protocol (UDP)
		0x00000000,
	)
	resp, err := doRPCTrans(conn, req, tId)
	if err != nil {
		return 0, fmt.Errorf("rstatd: rpcbind request failed: %v", err)
	}
	n := len(resp)
	if n < 4 {
		return 0, fmt.Errorf("rstatd: no respose from rpcbind")
	}

	port := binary.BigEndian.Uint32(resp[n-4 : n])
	if port == 0 {
		return 0, fmt.Errorf("rstatd: no port mapping found for rstatd")
	}
	return port, nil
}
