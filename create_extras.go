package sdp

import (
	"net"

	"github.com/emiago/sipgo/sip"
)

const (
	branchParam = "branch"
)

func CreateVIA(localSIPAddr *net.UDPAddr) *sip.ViaHeader {
	transport := "UDP"

	newVia := &sip.ViaHeader{
		ProtocolName:    "SIP",
		ProtocolVersion: "2.0",
		Transport:       transport,
		Host:            localSIPAddr.IP.String(),
		Port:            localSIPAddr.Port,
		Params:          sip.NewParams(),
	}
	newVia.Params.Add(branchParam, sip.GenerateBranch())
	newVia.Params.Add("rport", "")

	return newVia
}

func CreateACK(req *sip.Request, resp *sip.Response, localSIPAddr *net.UDPAddr) *sip.Request {
	contact := req.Contact().Address
	if resp.Contact() != nil {
		contact = resp.Contact().Address
	}

	reqToSend := sip.NewRequest(sip.ACK, contact)
	newVia := CreateVIA(localSIPAddr)
	if reqBranch, ok := req.Via().Params.Get(branchParam); ok {
		newVia.Params.Add(branchParam, reqBranch)
	}

	reqToSend.AppendHeader(newVia)
	reqToSend.AppendHeader(resp.From())
	reqToSend.AppendHeader(resp.To())
	reqToSend.AppendHeader(resp.CallID())
	reqToSend.AppendHeader(&sip.CSeqHeader{
		SeqNo:      req.CSeq().SeqNo,
		MethodName: sip.ACK,
	})
	reqToSend.AppendHeader(req.MaxForwards())
	reqToSend.AppendHeader(sip.NewHeader("User-Agent", UserAgent))
	reqToSend.AppendHeader(sip.NewHeader("Content-Length", "0"))

	return reqToSend
}

func CreateBYEtoUAS(reqInvite, lastACK *sip.Request, localSIPAddr *net.UDPAddr) *sip.Request {
	reqToSend := sip.NewRequest(sip.BYE, reqInvite.Contact().Address)
	reqToSend.SipVersion = reqInvite.SipVersion

	cseq := lastACK.CSeq()
	maxForwards := sip.MaxForwardsHeader(70)

	newVia := CreateVIA(localSIPAddr)
	reqToSend.AppendHeader(newVia)
	reqToSend.AppendHeader(sip.NewHeader("To", lastACK.From().Value()))
	reqToSend.AppendHeader(sip.NewHeader("From", lastACK.To().Value()))
	reqToSend.AppendHeader(lastACK.CallID())
	reqToSend.AppendHeader(&sip.CSeqHeader{
		SeqNo:      cseq.SeqNo + 1,
		MethodName: sip.BYE,
	})
	reqToSend.AppendHeader(&maxForwards)
	reqToSend.AppendHeader(sip.NewHeader("User-Agent", UserAgent))
	reqToSend.AppendHeader(sip.NewHeader("Content-Length", "0"))

	return reqToSend
}

func CreateBYEtoUAC(reqInvite, lastACK *sip.Request, localSIPAddr *net.UDPAddr) *sip.Request {
	reqToSend := sip.NewRequest(sip.BYE, reqInvite.Contact().Address)
	reqToSend.SipVersion = reqInvite.SipVersion

	cseq := lastACK.CSeq()
	maxForwards := sip.MaxForwardsHeader(70)

	newVia := CreateVIA(localSIPAddr)
	reqToSend.AppendHeader(newVia)
	reqToSend.AppendHeader(lastACK.To())
	reqToSend.AppendHeader(lastACK.From())
	reqToSend.AppendHeader(lastACK.CallID())
	reqToSend.AppendHeader(&sip.CSeqHeader{
		SeqNo:      cseq.SeqNo + 1,
		MethodName: sip.BYE,
	})
	reqToSend.AppendHeader(&maxForwards)
	reqToSend.AppendHeader(sip.NewHeader("User-Agent", UserAgent))
	reqToSend.AppendHeader(sip.NewHeader("Content-Length", "0"))

	return reqToSend
}

func CreateCANCELtoUAC(reqInvite *sip.Request, localSIPAddr *net.UDPAddr) *sip.Request {
	reqToSend := sip.NewRequest(sip.CANCEL, reqInvite.Recipient)

	cseq := reqInvite.CSeq()
	cseq.MethodName = sip.CANCEL

	newVia := reqInvite.Via()
	reqToSend.AppendHeader(newVia)
	reqToSend.AppendHeader(reqInvite.MaxForwards())
	reqToSend.AppendHeader(reqInvite.From())
	reqToSend.AppendHeader(reqInvite.To())
	reqToSend.AppendHeader(reqInvite.CallID())
	reqToSend.AppendHeader(reqInvite.CSeq())
	reqToSend.AppendHeader(sip.NewHeader("User-Agent", UserAgent))
	reqToSend.AppendHeader(sip.NewHeader("Content-Length", "0"))

	return reqToSend
}
