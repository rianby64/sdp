package sdp

import (
	"fmt"
	"math/rand"
	"net"

	"github.com/emiago/sipgo/sip"
	"github.com/google/uuid"
)

type UsersManager interface {
	GetAddr(user string) (net.Addr, bool)
}

// CreateINVITE creates an outgoing INVITE request to the user specified in the original request.
// It returns
//
// 1. RTP and RTCP connections,
// 2. the created INVITE request,
// 3. the address of the user the request should be sent to,
// 4. and an error if any.
func CreateINVITE(connSIP *net.UDPConn, rtpHost string, req *sip.Request, addrTo *net.UDPAddr) (*net.UDPConn, *net.UDPConn, *sip.Request, error) {
	inviteReq, connRTP, connRTCP, err := createInviteOutgoing(
		connSIP,
		rtpHost,
		req.MaxForwards().Val(),
		req.From(),
		req.To(),
		addrTo,
	)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("handling INVITE to the other user: %w", err)
	}

	return connRTP, connRTCP, inviteReq, nil
}

func createInviteOutgoing(
	connSIP *net.UDPConn,
	rtpHost string,
	reqMaxForwards uint32,
	headerFrom *sip.FromHeader,
	headerTo *sip.ToHeader,
	addrTo *net.UDPAddr,
) (*sip.Request, *net.UDPConn, *net.UDPConn, error) {
	localSIPAddr := connSIP.LocalAddr().(*net.UDPAddr)
	udpAddrTo := addrTo

	reqInviteTo := sip.NewRequest(sip.INVITE, sip.Uri{
		Scheme: "sip",
		User:   headerTo.Address.User,
		Host:   udpAddrTo.IP.String(),
		Port:   udpAddrTo.Port,
	})
	toSendSDP, connRTP, connRTCP, err := generateLocalSDP(connSIP, rtpHost)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("generating local SDP: %w", err)
	}

	reqInviteTo.SetBody(toSendSDP)

	via := CreateVIA(localSIPAddr)
	newcseq := &sip.CSeqHeader{
		SeqNo:      1 + uint32(rand.Intn(10000)),
		MethodName: sip.INVITE,
	}

	newCallId := sip.CallIDHeader(uuid.NewString())

	clonedTo := &sip.ToHeader{
		DisplayName: headerTo.DisplayName,
		Address: sip.Uri{
			Scheme: headerTo.Address.Scheme,
			User:   headerTo.Address.User,
			Host:   udpAddrTo.IP.String(),
			Port:   udpAddrTo.Port,
		},
		Params: sip.NewParams(),
	}

	contactHeader := &sip.ContactHeader{}
	contactHeader.Address = sip.Uri{
		Scheme: "sip",
		User:   headerFrom.Address.User,
		Host:   localSIPAddr.IP.String(),
		Port:   localSIPAddr.Port,
	}

	maxForwards := sip.MaxForwardsHeader(reqMaxForwards - 1)

	reqInviteTo.AppendHeader(via)
	reqInviteTo.AppendHeader(&newCallId)
	reqInviteTo.AppendHeader(newcseq)
	reqInviteTo.AppendHeader(headerFrom)
	reqInviteTo.AppendHeader(clonedTo)
	reqInviteTo.AppendHeader(contactHeader)
	reqInviteTo.AppendHeader(&maxForwards)
	reqInviteTo.AppendHeader(sip.NewHeader("Allow", "PRACK,INVITE,ACK,BYE,CANCEL,UPDATE,INFO,SUBSCRIBE,NOTIFY,REFER,MESSAGE,OPTIONS"))
	reqInviteTo.AppendHeader(sip.NewHeader("Supported", "replaces,100rel,timer,norefersub"))
	reqInviteTo.AppendHeader(sip.NewHeader("User-Agent", UserAgent))
	reqInviteTo.AppendHeader(sip.NewHeader("Content-Type", "application/sdp"))
	reqInviteTo.AppendHeader(sip.NewHeader("Accept", "application/sdp"))

	return reqInviteTo, connRTP, connRTCP, nil
}
