package udp

import (
	"testing"
	"time"
	"network/ipv4/ipv4tps"
	"network/ipv4/ipv4src"
)

const rwport = 20102

func TestReadWriteLocal(t *testing.T) {
	read_write_test(t, ipv4src.Loopback_ip_address)
}

func TestReadWriteExternal(t *testing.T) {
	t.Skip("External tests actually don't work")
	read_write_test(t, ipv4src.External_ip_address)
}

func read_write_test(t *testing.T, ip *ipv4tps.IPaddress) {
	success := make(chan bool, 1)

	r, err := NewUDP(GlobalUDP_Read_Manager, rwport, ip)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()

	data := []byte{'h', 'e', 'l', 'l', 'o'}

	go func() {
		w, err := NewUDP_Writer(20000, rwport, ip)
		if err != nil {
			t.Fatal(err)
		}

		err = w.Write(data)
		if err != nil {
			t.Fatal(err)
		} else {
			t.Log("Wrote the data:", data)
		}

		w.Close()
	}()

	go func() {
		//time.Sleep(10*time.Second)
		p, err := r.Read(MAX_UDP_PACKET_LEN)
		if err != nil {
			t.Fatal(err)
		}
		t.Log("Output:", string(p))

		if string(p) == string(data) {
			t.Log("Got correct output:", p)
			success <- true
		} else {
			t.Error("Got Wrong Output:", p)
		}
	}()

	select {
	case <-success:
		t.Log("Success")
	case <-time.After(5 * time.Second):
		t.Error("Timed out")
	}
}
