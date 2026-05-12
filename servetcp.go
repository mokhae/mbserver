package mbserver

import (
	"crypto/tls"
	"io"
	"log"
	"net"
	"strings"
	"time"
)

type tcpSessionState int

const (
	tcpSessionActive tcpSessionState = iota
	tcpSessionGrace
)

type tcpSession struct {
	remoteIP   string
	conn       net.Conn
	state      tcpSessionState
	graceTimer *time.Timer
}

func extractRemoteIP(addr net.Addr) string {
	host, _, err := net.SplitHostPort(addr.String())
	if err != nil {
		return addr.String()
	}
	return host
}

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

		remoteIP := extractRemoteIP(conn.RemoteAddr())

		var session *tcpSession
		isReconnect := false

		if s.disconnectTimeout > 0 {
			s.tcpSessionsMu.Lock()
			session = s.tcpSessions[remoteIP]

			if session != nil && session.state == tcpSessionGrace {
				// Reconnect within grace period — cancel timer, restore session
				session.graceTimer.Stop()
				session.graceTimer = nil
				session.state = tcpSessionActive
				session.conn = conn
				isReconnect = true
			} else {
				session = &tcpSession{
					remoteIP: remoteIP,
					conn:     conn,
					state:    tcpSessionActive,
				}
				s.tcpSessions[remoteIP] = session
			}
			s.tcpSessionsMu.Unlock()
		}

		s.clientConns.Store(conn, struct{}{})
		s.watchdog.Feed(conn)

		if !isReconnect && accectCallback != nil {
			accectCallback(conn)
		}

		go func(conn net.Conn, session *tcpSession) {
			defer func() {
				s.clientConns.Delete(conn)
				s.watchdog.Remove(conn)
				conn.Close()

				if session != nil {
					// disconnectTimeout > 0: start grace period
					s.tcpSessionsMu.Lock()
					if session.conn == conn {
						session.state = tcpSessionGrace
						session.conn = nil
						session.graceTimer = time.AfterFunc(s.disconnectTimeout, func() {
							s.tcpSessionsMu.Lock()
							if s.tcpSessions[session.remoteIP] == session && session.state == tcpSessionGrace {
								delete(s.tcpSessions, session.remoteIP)
								s.tcpSessionsMu.Unlock()
								if disCallback != nil {
									disCallback(conn)
								}
							} else {
								s.tcpSessionsMu.Unlock()
							}
						})
					}
					// else: new conn already took over this session — do nothing
					s.tcpSessionsMu.Unlock()
				} else {
					// disconnectTimeout == 0: immediate cleanup, original behavior
					if disCallback != nil {
						disCallback(conn)
					}
				}
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
				packet = packet[:bytesRead]

				frame, err := NewTCPFrame(packet, conn)
				if err != nil {
					log.Printf("bad packet error %v\n", err)
					return
				}

				s.watchdog.Feed(conn)

				request := &Request{conn, frame}

				s.requestChan <- request
				<-s.responseChan
			}
		}(conn, session)
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
