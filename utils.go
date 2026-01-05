package sdp

import (
	"fmt"
	"net"

	"github.com/emiago/sipgo/sip"
)

type Param interface {
	Request() *sip.Request
	Response() *sip.Response
	UDPConnSIP() *net.UDPConn
	RemoteUDPAddr() *net.UDPAddr
}

func WriteResponse(param Param) error {
	resp := param.Response()
	conn := param.UDPConnSIP()
	addr := param.RemoteUDPAddr()
	payload := []byte(resp.String())

	_, err := conn.WriteTo(payload, addr)
	if err != nil {
		return fmt.Errorf("failed to send %d %s response: %w", resp.StatusCode, resp.Reason, err)
	}

	return nil
}

func WriteRequest(param Param) error {
	req := param.Request()
	conn := param.UDPConnSIP()
	addr := param.RemoteUDPAddr()
	payload := []byte(req.String())

	_, err := conn.WriteTo(payload, addr)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}

	return nil
}
