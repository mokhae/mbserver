package mbserver

import (
	"crypto/tls"
	"io"
	"log"
	"net"
	"strings"
)

func (s *Server) accept(listen net.Listener, accectCallback ListenCallback, disCallback DisconnectCallback) error {
	for {
		conn, err := listen.Accept()
		if err != nil {
			if strings.Contains(err.Error(), "use of closed network connection") {
				return nil
			}
			log.Printf("Unable to accept connections: %#v\n", err)
			return err
		}

		// Watchdog
		s.clientConns.Store(conn, struct{}{})
		s.watchdog.Feed(conn)
		if accectCallback != nil {
			accectCallback(conn)
		}

		go func(conn net.Conn) {
			defer func() {
				log.Printf("Client disconnected: %v", conn.RemoteAddr())
				s.clientConns.Delete(conn)
				s.watchdog.Remove(conn)
				if disCallback != nil {
					disCallback(conn)
				}
				conn.Close()

			}()

			for {
				packet := make([]byte, 512)
				bytesRead, err := conn.Read(packet)
				if err != nil {
					if err != io.EOF {
						log.Printf("read error %v\n", err)
					}
					return
				}
				// Set the length of the packet to the number of read bytes.
				packet = packet[:bytesRead]

				frame, err := NewTCPFrame(packet, conn)
				if err != nil {
					log.Printf("bad packet error %v\n", err)
					return
				}

				request := &Request{conn, frame}

				s.requestChan <- request
			}
		}(conn)
	}
}

// ListenTCP starts the Modbus server listening on "address:port".
func (s *Server) ListenTCP(addressPort string) (err error) {
	listen, err := net.Listen("tcp", addressPort)
	if err != nil {
		log.Printf("Failed to Listen: %v\n", err)
		return err
	}
	s.listeners = append(s.listeners, listen)
	go s.accept(listen, nil, nil)
	return err
}

func (s *Server) ListenTCPCallback(addressPort string, accectCallback ListenCallback, disCallback DisconnectCallback) (err error) {
	listen, err := net.Listen("tcp", addressPort)
	if err != nil {
		log.Printf("Failed to Listen: %v\n", err)
		return err
	}
	s.listeners = append(s.listeners, listen)
	go s.accept(listen, accectCallback, disCallback)
	return err
}

// ListenTLS starts the Modbus server listening on "address:port".
func (s *Server) ListenTLS(addressPort string, config *tls.Config) (err error) {
	listen, err := tls.Listen("tcp", addressPort, config)
	if err != nil {
		log.Printf("Failed to Listen on TLS: %v\n", err)
		return err
	}
	s.listeners = append(s.listeners, listen)
	go s.accept(listen, nil, nil)
	return err
}
