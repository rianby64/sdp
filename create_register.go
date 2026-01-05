package sdp

import (
	"fmt"
	"math/rand/v2"
	"net"

	"github.com/emiago/sipgo/sip"
	"github.com/google/uuid"
	"github.com/icholy/digest"
)

const (
	scheme = "sip"
	maxV   = 1 << 16
)

type Credentials struct {
	Username,
	password,
	Host string
	Port int
}

func (c *Credentials) SetPassword(password string) {
	c.password = password
}

type WWWAuthenticate struct {
	Scheme,
	Realm,
	Nonce,
	Algorithm string
	StaleFlag bool
}

type Authorization struct {
	Scheme,
	Username,
	Realm,
	Nonce,
	URI,
	Digest,
	Algorithm string
}

func AuthRequest(wwwAuth string, creds *Credentials, req *sip.Request) (*sip.Request, error) {
	cseq := req.CSeq()
	cseq.SeqNo++

	reqAuthenticated := req.Clone()
	reqAuthenticated.ReplaceHeader(cseq)

	if creds == nil {
		return nil, fmt.Errorf("no creds")
	}

	if creds.Host == "" || creds.Username == "" || creds.password == "" {
		return reqAuthenticated, nil
	}

	if wwwAuth == "" {
		return nil, fmt.Errorf("www-authenticate header required")
	}

	challenge, err := digest.ParseChallenge(wwwAuth)
	if err != nil {
		return nil, fmt.Errorf("failed to parse challenge wwwauth %s: %w", wwwAuth, err)
	}

	solution, err := digest.Digest(challenge, digest.Options{
		Method:   req.Method.String(),
		URI:      creds.Host,
		Username: creds.Username,
		Password: creds.password,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to solve challenge wwwauth %s: %w", wwwAuth, err)
	}

	reqAuthenticated.AppendHeader(sip.NewHeader("Authorization", solution.String()))

	return reqAuthenticated, nil
}

func CreateREGISTER(creds *Credentials, callID string, lAddr *net.UDPAddr) (*sip.Request, error) {
	reqRegister := sip.NewRequest(sip.REGISTER, sip.Uri{
		Scheme: scheme,
		Host:   creds.Host,
		Port:   creds.Port,
	})

	via := CreateVIA(lAddr)
	maxForwards := sip.NewHeader("Max-Forwards", "70")
	route := &sip.RouteHeader{
		Address: sip.Uri{
			Scheme:    scheme,
			Host:      creds.Host,
			Port:      creds.Port,
			UriParams: sip.NewParams().Add("lr", ""),
		},
	}
	cseq := &sip.CSeqHeader{
		SeqNo:      1 + rand.Uint32N(maxV),
		MethodName: sip.REGISTER,
	}
	expires := sip.ExpiresHeader(300)
	contact := &sip.ContactHeader{
		Address: sip.Uri{
			Scheme:    scheme,
			Host:      lAddr.IP.String(),
			Port:      lAddr.Port,
			User:      creds.Username,
			UriParams: sip.NewParams().Add("ob", ""),
		},
	}
	newCallId := sip.CallIDHeader(uuid.NewString())
	if callID != "" {
		newCallId = sip.CallIDHeader(callID)
	}

	from := &sip.FromHeader{
		Address: sip.Uri{
			Scheme: scheme,
			Host:   creds.Host,
			User:   creds.Username,
		},
		Params: sip.NewParams().Add("tag", uuid.NewString()),
	}
	to := &sip.ToHeader{
		Address: sip.Uri{
			Scheme: scheme,
			Host:   creds.Host,
			User:   creds.Username,
		},
	}

	reqRegister.AppendHeader(via)
	reqRegister.AppendHeader(maxForwards)
	reqRegister.AppendHeader(route)
	reqRegister.AppendHeader(cseq)
	reqRegister.AppendHeader(sip.NewHeader("User-Agent", UserAgent))
	reqRegister.AppendHeader(&expires)
	reqRegister.AppendHeader(sip.NewHeader("Allow", "PRACK, INVITE, ACK, BYE, CANCEL, UPDATE, INFO, SUBSCRIBE, NOTIFY, REFER, MESSAGE, OPTIONS"))
	reqRegister.AppendHeader(contact)
	reqRegister.AppendHeader(&newCallId)
	reqRegister.AppendHeader(from)
	reqRegister.AppendHeader(to)

	return reqRegister, nil
}
