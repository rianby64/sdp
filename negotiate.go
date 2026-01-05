package sdp

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/emiago/sipgo/sip"
	"github.com/pion/sdp/v4"
)

type UDPConn interface {
	LocalAddr() net.Addr
	Close() error
}

func RenegotiateSDP(req *sip.Request, connSIP, connRTP, connRTCP UDPConn) (*sip.Response, string, int, error) {
	body := req.Body()
	if len(body) == 0 {
		return nil, "", 0, fmt.Errorf("no SDP in the request")
	}

	remoteSDP := &sdp.SessionDescription{}
	if err := remoteSDP.Unmarshal(body); err != nil {
		return nil, "", 0, fmt.Errorf("parsing remote SDP: %w", err)
	}
	localSDP, selectedFormat := negotiateLocalSDP(remoteSDP, connSIP, connRTP, connRTCP)

	resp, err := createSDPResponse(localSDP, req, connSIP)
	if err != nil {
		errFinal := fmt.Errorf("creating SDP response: %w", err)
		if err := connRTP.Close(); err != nil {
			errFinal = fmt.Errorf("%w; closing RTP connection: %w", errFinal, err)
		}

		if err := connRTCP.Close(); err != nil {
			errFinal = fmt.Errorf("%w; closing RTCP connection: %w", errFinal, err)
		}

		return nil, "", 0, errFinal
	}

	return resp, selectedFormat, ptimeDefault, nil
}

func NegotiateSDP(req *sip.Request, connSIP UDPConn) (*sip.Response, *net.UDPConn, *net.UDPConn, string, int, error) {
	connRTP, connRTCP, err := generateNewRTPAndRTCP(connSIP)
	if err != nil {
		return nil, nil, nil, "", 0, fmt.Errorf("generating RTP and RTCP connections: %w", err)
	}

	resp, selectedFormat, pTime, err := RenegotiateSDP(req, connSIP, connRTP, connRTCP)
	if err != nil {
		return nil, nil, nil, "", 0, fmt.Errorf("negotiating SDP: %w", err)
	}

	return resp, connRTP, connRTCP, selectedFormat, pTime, nil
}

func ObtainSelectedFormatAndPtime(body []byte) (string, int, *net.UDPAddr, *net.UDPAddr, error) {
	var (
		addrToRTCP,
		addrToRTP *net.UDPAddr
	)

	remoteSDP, err := unmarshalSDP(body)
	if err != nil {
		return "", 0, nil, nil, fmt.Errorf("unmarshaling SDP: %w", err)
	}

	if len(remoteSDP.MediaDescriptions) == 0 {
		return "", 0, nil, nil, fmt.Errorf("no media descriptions in SDP")
	}

	addrRTP := net.ParseIP(remoteSDP.Origin.UnicastAddress)
	addrToRTP = &net.UDPAddr{
		IP:   addrRTP,
		Port: remoteSDP.MediaDescriptions[0].MediaName.Port.Value,
	}

	fmt.Printf("remoteSDP.Origin.UnicastAddress %s\n", remoteSDP.Origin.UnicastAddress)

	if remoteSDP.ConnectionInformation != nil {
		fmt.Printf("remoteSDP.ConnectionInformation.Address %s\n", remoteSDP.ConnectionInformation.Address)
		addrRTP := net.ParseIP(remoteSDP.ConnectionInformation.Address.Address)
		addrToRTP = &net.UDPAddr{
			IP:   addrRTP,
			Port: remoteSDP.MediaDescriptions[0].MediaName.Port.Value,
		}
	}

	if len(remoteSDP.MediaDescriptions[0].MediaName.Formats) == 0 {
		return "", 0, nil, nil, fmt.Errorf("no formats in media description")
	}

	pTime := int(defFrameDur / time.Millisecond)

	for _, attr := range remoteSDP.MediaDescriptions[0].Attributes {
		if attr.Key == ptimeHeader {
			v, err := strconv.Atoi(attr.Value)
			if err == nil {
				pTime = v
			}

			continue
		}

		if attr.Key == rtcpHeader {
			v := strings.Split(attr.Value, " ")
			if len(v) > 0 {
				port, err := strconv.Atoi(v[0])
				if err != nil {
					addrToRTCP = nil

					continue
				}

				addrToRTCP = &net.UDPAddr{
					Port: port,
				}
			}

			if len(v) > 3 {
				addrToRTCP.IP = net.ParseIP(v[3])
			}
		}
	}

	return remoteSDP.MediaDescriptions[0].MediaName.Formats[0], pTime, addrToRTP, addrToRTCP, nil
}
