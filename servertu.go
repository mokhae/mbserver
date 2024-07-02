package mbserver

import (
	"io"
	"log"
	"strings"

	"github.com/goburrow/serial"
)

// ListenRTU starts the Modbus server listening to a serial device.
// For example:  err := s.ListenRTU(&serial.Config{Address: "/dev/ttyUSB0"})
func (s *Server) ListenRTU(serialConfig *serial.Config, deviceId uint8) (err error) {
	port, err := serial.Open(serialConfig)
	if err != nil {
		//log.Fatalf("failed to open %s: %v\n", serialConfig.Address, err)
		log.Printf("failed to open %s: %v\n", serialConfig.Address, err)
		return err
	}
	s.ports = append(s.ports, port)

	s.portsWG.Add(1)
	go func() {
		defer s.portsWG.Done()
		s.acceptSerialRequests(port, deviceId)
	}()

	return err
}

func (s *Server) acceptSerialRequests(port serial.Port, deviceId uint8) {

	Abuf := make([]byte, 0)
SkipFrameError:
	for {
		select {
		case <-s.portsCloseChan:

			log.Printf("Port Closed\n")
			return
		default:
		}

		buffer := make([]byte, 512)

		bytesRead, err := port.Read(buffer)
		if err != nil {
			if err != io.EOF {
				log.Printf("serial read error %v\n", err)
			}
			continue SkipFrameError
		}

		if bytesRead != 0 {

			// Set the length of the packet to the number of read bytes.
			packet := buffer[:bytesRead]
			res1 := append(Abuf, packet...)
			//if len(Abuf) > 20 {
			//	Abuf = make([]byte, 0)
			//}
			frame, err := NewRTUFrame(res1)
			if err != nil {
				if strings.Contains(err.Error(), "RTU Frame error: CRC") {
					Abuf = make([]byte, 0)

				} else {
					Abuf = res1
				}
				log.Printf("bad serial frame error %v\n", err)
				//The next line prevents RTU server from exiting when it receives a bad frame. Simply discard the erroneous
				//frame and wait for next frame by jumping back to the beginning of the 'for' loop.
				log.Printf("Keep the RTU server running!!\n")
				continue SkipFrameError
				//return
			}
			log.Printf("New Frame : %v\n", frame)
			Abuf = make([]byte, 0)

			if frame.GetAddress() == deviceId {
				request := &Request{port, frame}
				s.requestChan <- request
			} else {
				log.Printf("wrong slave address")
			}
		}
	}

}
