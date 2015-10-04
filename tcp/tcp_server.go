package tcp

import (
	"errors"
	"network/ipv4"
	"sync"

	"github.com/hsheth2/logs"
)

type Server_TCB struct {
	listener        chan *packet
	listenPort      uint16
	listenIP        *ipv4.Address
	state           fsmState
	kind            tcbParentType
	connQueue       chan *TCB
	connQueueUpdate *sync.Cond
}

func New_Server_TCB() (*Server_TCB, error) {
	x := &Server_TCB{
		listener:        nil,
		listenPort:      0,
		listenIP:        ipv4.IPAll,
		state:           fsmClosed,
		kind:            serverParent,
		connQueueUpdate: sync.NewCond(&sync.Mutex{}),
	}

	return x, nil
}

func (s *Server_TCB) BindListen(port uint16, ip *ipv4.Address) error {
	return s.BindListenWithQueueSize(port, ip, TCP_LISTEN_DEFAULT_QUEUE_SZ)
}

func (s *Server_TCB) BindListenWithQueueSize(port uint16, ip *ipv4.Address, queue_sz int) error {
	s.listenPort = port
	s.listenIP = ip
	read, err := TCP_Port_Manager.bind(0, port, ip)
	if err != nil {
		return err
	}
	s.listener = read
	s.state = fsmListen
	s.connQueue = make(chan *TCB, queue_sz)

	go s.LongListener()

	return nil
}

func (s *Server_TCB) LongListener() {
	//ch logs.Trace.Println("Server listener routine")
	for {
		in := <-s.listener
		////ch logs.Trace.Println("Server rcvd packet:", in)
		if in.header.flags&flagRst != 0 {
			continue // parent TCB drops the RST
		} else if in.header.flags&flagAck != 0 {
			// TODO send reset
		} else if in.header.flags&flagSyn == 0 {
			// TODO send reset
		}

		////ch logs.Trace.Println("Packet rcvd by server has promise: responding with SYN-ACK")
		go func(s *Server_TCB, in *packet) {
			lp := s.listenPort
			rp := in.header.srcport
			rIP := in.rip

			read, err := TCP_Port_Manager.bind(rp, lp, rIP)
			if err != nil {
				logs.Error.Println(err)
				return
			}

			r, err := ipv4.NewWriter(rIP, ipv4.IPProtoTCP)
			if err != nil {
				logs.Error.Println(err)
				return
			}

			c, err := New_TCB(lp, rp, rIP, read, r, serverParent)
			if err != nil {
				logs.Error.Println(err)
				return
			}
			c.serverParent = s

			// update state
			c.updateState(fsmSynRcvd)

			// send syn-ack
			c.ackNum = in.header.seq + 1
			//ch logs.Trace.Printf("%s Server/TCB seq: %d, ack: %d, to rip: %v\n", c.Hash(), c.seqNum, c.ackNum, c.ipAddress.IP)
			synack := &packet{
				header: &header{
					seq:     c.seqNum,
					ack:     c.ackNum,
					flags:   flagSyn | flagAck,
					urg:     0,
					options: []byte{0x02, 0x04, 0xff, 0xd7, 0x04, 0x02, 0x08, 0x0a, 0x02, 0x64, 0x80, 0x8b, 0x0, 0x0, 0x0, 0x0, 0x01, 0x03, 0x03, 0x07},
					// TODO compute the options of Syn-Ack instead of hardcoding them
				},
				payload: []byte{},
			}
			// TODO make sure that the seq and ack numbers are set properly
			c.seqAckMutex.Lock()
			c.seqNum += 1
			c.seqAckMutex.Unlock()
			//ch logs.Trace.Println(c.Hash(), "Server/TCB about to respond with SYN-ACK")
			err = c.sendWithRetransmit(synack)
			if err != nil {
				logs.Error.Println(err)
				return
			}
			//ch logs.Trace.Println(c.Hash(), "Server/TCB responded with SYN-ACK")

			select {
			case s.connQueue <- c:
			default:
				// TODO send a reset or expand queue
				logs.Error.Println(errors.New("ERR: listen queue is full"))
				return
			}
			return
		}(s, in)
	}
}

func (s *Server_TCB) Accept() (c *TCB, rip *ipv4.Address, rport uint16, err error) {
	s.connQueueUpdate.L.Lock()
	defer s.connQueueUpdate.L.Unlock()
	for {
		// TODO add a timeout and remove the inner loop
		for i := 0; i < len(s.connQueue); i++ {
			next := <-s.connQueue
			if next.getState() == fsmEstablished {
				return next, next.ipAddress, next.rport, nil
			}
			s.connQueue <- next
		}
		s.connQueueUpdate.Wait()
	}
}

func (s *Server_TCB) Close() error {
	return nil // TODO actually close the server tcb
}
