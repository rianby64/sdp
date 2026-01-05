package sdp

import (
	"fmt"
	"math/rand"
	"net"
	"time"

	"github.com/emiago/sipgo/sip"
	"github.com/pion/sdp/v4"
)

const (
	UserAgent    = "Andres/0.1"
	defFrameDur  = time.Duration(20 * time.Millisecond) // 20ms
	rtcpHeader   = "rtcp"
	ptimeHeader  = "ptime"
	ptimeDefault = int(defFrameDur / time.Millisecond)
)

func createContactHeader(connSIP UDPConn) *sip.ContactHeader {
	contact := &sip.ContactHeader{}
	contact.Address.Host = connSIP.LocalAddr().(*net.UDPAddr).IP.String()
	contact.Address.Port = connSIP.LocalAddr().(*net.UDPAddr).Port
	contact.Address.User = "andres-proxy"

	return contact
}

var (
	availableCodecs = map[string]sdp.Attribute{
		"0": {
			Key:   "rtpmap",
			Value: "0 PCMU/8000",
		},
		"8": {
			Key:   "rtpmap",
			Value: "8 PCMA/8000",
		},
		"9": {
			Key:   "rtpmap",
			Value: "9 G722/8000",
		},
		"96": {
			Key:   "rtpmap",
			Value: "96 opus/48000/2",
		},
		"105": {
			Key:   "rtpmap",
			Value: "105 opus/48000/2",
		},
		"106": {
			Key:   "rtpmap",
			Value: "106 opus/48000/2",
		},
	}
	ConfigCodecs = map[string]struct {
		SamplingRate int
		Channels     int
		PayloadType  byte
	}{
		"0":   {8000, 1, 0},
		"8":   {8000, 1, 8},
		"9":   {8000, 1, 9}, // TODO: G722 - not ready. Bad audio from LK to SIP
		"96":  {48000, 1, 96},
		"105": {48000, 1, 105},
		"106": {48000, 1, 106},
	}
)

func createConnRTP(connSIP UDPConn) (*net.UDPConn, error) {
	laddrRTP := &net.UDPAddr{
		IP:   connSIP.LocalAddr().(*net.UDPAddr).IP,
		Port: 0,
		Zone: connSIP.LocalAddr().(*net.UDPAddr).Zone,
	}

	connRTP, err := net.ListenUDP("udp", laddrRTP)
	if err != nil {
		return nil, fmt.Errorf("cannot start RTP listener: %w", err)
	}

	return connRTP, nil
}

func createConnRTCP(connSIP, connRTP UDPConn) (*net.UDPConn, error) {
	laddrRTPport := connRTP.LocalAddr().(*net.UDPAddr).Port + 1
	laddrRTCP := &net.UDPAddr{
		IP:   connSIP.LocalAddr().(*net.UDPAddr).IP,
		Port: laddrRTPport,
		Zone: connSIP.LocalAddr().(*net.UDPAddr).Zone,
	}

	connRTCP, err := net.ListenUDP("udp", laddrRTCP)
	if err != nil {
		return nil, fmt.Errorf("cannot start RTP listener: %w", err)
	}

	return connRTCP, nil
}

func generateNewRTPAndRTCP(connSIP UDPConn) (*net.UDPConn, *net.UDPConn, error) {
	connRTP, err := createConnRTP(connSIP)
	if err != nil {
		return nil, nil, fmt.Errorf("creating RTP connection: %w", err)
	}

	connRTCP, err := createConnRTCP(connSIP, connRTP)
	if err != nil {
		return nil, nil, fmt.Errorf("creating RTCP connection: %w", err)
	}

	return connRTP, connRTCP, nil
}

func createSDPResponse(localSDP *sdp.SessionDescription, req *sip.Request, connSIP UDPConn) (*sip.Response, error) {
	data, err := localSDP.Marshal()
	if err != nil {
		return nil, fmt.Errorf("failed to marshal local SDP: %v", err)
	}

	sdpResp := sip.NewSDPResponseFromRequest(req, data)
	sdpResp.AppendHeader(createContactHeader(connSIP))

	return sdpResp, nil
}

func generateLocalSDP(connSIP *net.UDPConn, rtpHost string) ([]byte, *net.UDPConn, *net.UDPConn, error) {
	rtpAddr := net.ParseIP(rtpHost)
	temporaryConnRTP, err := net.ListenUDP("udp", &net.UDPAddr{IP: rtpAddr})
	if err != nil {
		panic(err)
	}

	defer temporaryConnRTP.Close()

	connRTP, connRTCP, err := generateNewRTPAndRTCP(temporaryConnRTP)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("generating RTP and RTCP connections: %w", err)
	}

	ownSDP := &sdp.SessionDescription{
		MediaDescriptions: []*sdp.MediaDescription{
			{
				MediaName: sdp.MediaName{
					Media: "audio",
					Formats: []string{
						"106", "105", "96", "8", "0",
					},
				},
			},
		},
	}

	sdp, _ := negotiateLocalSDP(ownSDP, connSIP, connRTP, connRTCP)

	data, err := sdp.Marshal()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("marshaling local SDP: %w", err)
	}

	return data, connRTP, connRTCP, nil
}

func negotiateLocalSDP(
	remoteSDP *sdp.SessionDescription,
	_ UDPConn,
	connRTP UDPConn,
	connRTCP UDPConn,
) (*sdp.SessionDescription, string) {
	// connSIPLocalAddr := connSIP.LocalAddr().(*net.UDPAddr)
	connRTPLocalAddr := connRTP.LocalAddr().(*net.UDPAddr)
	connRTCPLocalAddr := connRTCP.LocalAddr().(*net.UDPAddr)

	localSDP := &sdp.SessionDescription{}
	localSDP.Origin.Username = "-"
	localSDP.Origin.SessionID = uint64(rand.Uint32())
	localSDP.Origin.SessionVersion = uint64(rand.Uint32())
	localSDP.Origin.UnicastAddress = connRTPLocalAddr.IP.String()
	localSDP.Origin.AddressType = obtainAdressType(connRTPLocalAddr.IP)
	localSDP.Origin.NetworkType = "IN"
	localSDP.SessionName = "Andres-RTP"
	localSDP.TimeDescriptions = []sdp.TimeDescription{
		{
			Timing: sdp.Timing{
				StartTime: 0,
				StopTime:  0,
			},
		},
	}

	selectedFormat := ""
	formats := []string{}
	mediaAttributes := []sdp.Attribute{}

	if len(remoteSDP.MediaDescriptions) > 0 {
		for _, format := range remoteSDP.MediaDescriptions[0].MediaName.Formats {
			if attr, ok := availableCodecs[format]; ok {
				formats = append(formats, format)
				mediaAttributes = append(mediaAttributes, attr)
			}
		}
		if len(formats) > 0 {
			selectedFormat = formats[0]
		}
	}

	localSDP.ConnectionInformation = &sdp.ConnectionInformation{
		NetworkType: "IN",
		AddressType: obtainAdressType(connRTPLocalAddr.IP),
		Address:     &sdp.Address{Address: connRTPLocalAddr.IP.String()},
	}

	localSDP.MediaDescriptions = []*sdp.MediaDescription{
		{
			MediaName: sdp.MediaName{
				Media:   "audio",
				Port:    sdp.RangedPort{Value: connRTPLocalAddr.Port},
				Protos:  []string{"RTP", "AVP"},
				Formats: formats,
			},
			Attributes: append(mediaAttributes, []sdp.Attribute{
				{
					Key:   ptimeHeader,
					Value: fmt.Sprint(ptimeDefault),
				},
				{
					Key:   "minptime",
					Value: "10",
				},
				{
					Key:   "sendrecv",
					Value: "",
				},
				{
					Key:   "rtcp",
					Value: fmt.Sprintf("%d IN %s %s", connRTCPLocalAddr.Port, obtainAdressType(connRTCPLocalAddr.IP), connRTCPLocalAddr.IP.String()),
				},
			}...),
		},
	}

	return localSDP, selectedFormat
}

func unmarshalSDP(data []byte) (*sdp.SessionDescription, error) {
	remoteSDP := &sdp.SessionDescription{}
	if err := remoteSDP.Unmarshal(data); err != nil {
		return nil, fmt.Errorf("parsing remote SDP: %w", err)
	}

	return remoteSDP, nil
}

func obtainAdressType(ip net.IP) string {
	if ip.To4() != nil {
		return "IP4"
	}
	return "IP6"
}
