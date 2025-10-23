// Package mbserver implments a Modbus server (slave).
package mbserver

import (
	"io"
	"log"
	"net"
	"sync"
	"time"

	"github.com/goburrow/serial"
)

// Server is a Modbus slave with allocated memory for discrete inputs, coils, etc.
type Server struct {
	// Debug enables more verbose messaging.
	Debug            bool
	listeners        []net.Listener
	clientConns      sync.Map
	watchdog         *Watchdog
	ports            []serial.Port
	portsWG          sync.WaitGroup
	portsCloseChan   chan struct{}
	requestChan      chan *Request
	responseChan     chan bool
	Function         [256](func(*Server, Framer) ([]byte, *Exception))
	DiscreteInputs   []byte
	Coils            []byte
	HoldingRegisters []uint16
	InputRegisters   []uint16
}

// Request contains the connection and Modbus frame.
type Request struct {
	conn  io.ReadWriteCloser
	frame Framer
}

type ListenCallback func(conn net.Conn)
type DisconnectCallback func(conn net.Conn)
type PortErrorCallback func(err error)

// NewServer creates a new Modbus server (slave).
func NewServer(wdFlag bool, wdTimeout time.Duration) *Server {
	s := &Server{}

	// Allocate Modbus memory maps.
	s.DiscreteInputs = make([]byte, 65536)
	s.Coils = make([]byte, 65536)
	s.HoldingRegisters = make([]uint16, 65536)
	s.InputRegisters = make([]uint16, 65536)

	// Add default functions.
	s.Function[1] = ReadCoils
	s.Function[2] = ReadDiscreteInputs
	s.Function[3] = ReadHoldingRegisters
	s.Function[4] = ReadInputRegisters
	s.Function[5] = WriteSingleCoil
	s.Function[6] = WriteHoldingRegister
	s.Function[15] = WriteMultipleCoils
	s.Function[16] = WriteHoldingRegisters

	s.requestChan = make(chan *Request)
	s.responseChan = make(chan bool)
	s.portsCloseChan = make(chan struct{})

	s.watchdog = NewWatchdog(wdTimeout, func(conn net.Conn) {
		log.Printf("Watchdog timeout: no requests received from %v in the last %v", conn.RemoteAddr(), wdTimeout)
		conn.Close()
		s.clientConns.Delete(conn)
	})

	if wdFlag {
		s.watchdog.Start()
	}

	go s.handler()

	return s
}

// RegisterFunctionHandler override the default behavior for a given Modbus function.
func (s *Server) RegisterFunctionHandler(funcCode uint8, function func(*Server, Framer) ([]byte, *Exception)) {
	s.Function[funcCode] = function
}

func (s *Server) handle(request *Request) Framer {
	var exception *Exception
	var data []byte

	response := request.frame.Copy()

	function := request.frame.GetFunction()
	if s.Function[function] != nil {
		data, exception = s.Function[function](s, request.frame)
		response.SetData(data)

		//s.watchdog.Feed(request.frame.Conn())
	} else {
		exception = &IllegalFunction
	}

	if exception != &Success {
		response.SetException(exception)
	}

	return response
}

// All requests are handled synchronously to prevent modbus memory corruption.
func (s *Server) handler() {
	for {
		request := <-s.requestChan
		response := s.handle(request)
		//log.Printf("Response : %v", response.Bytes())
		_, err := request.conn.Write(response.Bytes())
		if err != nil {
			log.Printf("Write error: %v", err)
		}

		//log.Printf("Write %v bytes to client", n)
		s.responseChan <- true
	}
}

// Close stops listening to TCP/IP ports and closes serial ports.
func (s *Server) Close() {
	for _, listen := range s.listeners {
		listen.Close()
	}

	close(s.portsCloseChan)
	s.portsWG.Wait()

	for _, port := range s.ports {
		port.Close()
	}
}
