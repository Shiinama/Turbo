package socks

import (
	"errors"
	"fmt"
	"io"
	"net"
	"server/database"
)

type AuthResult struct {
	UserID int64
	NodeID string
}

func Authenticate(conn net.Conn) (*AuthResult, error) {
	// Read auth version (must be 0x01)
	header := make([]byte, 1)
	if _, err := io.ReadFull(conn, header); err != nil {
		return nil, err
	}

	if header[0] != 0x01 {
		return nil, errors.New("unsupported auth version")
	}

	userLenBuf := make([]byte, 1)
	if _, err := io.ReadFull(conn, userLenBuf); err != nil {
		return nil, err
	}
	userLen := userLenBuf[0]

	userBuf := make([]byte, userLen)
	if _, err := io.ReadFull(conn, userBuf); err != nil {
		return nil, err
	}
	username := string(userBuf)

	passLenBuf := make([]byte, 1)
	if _, err := io.ReadFull(conn, passLenBuf); err != nil {
		return nil, err
	}
	passLen := passLenBuf[0]

	passBuf := make([]byte, passLen)
	if _, err := io.ReadFull(conn, passBuf); err != nil {
		return nil, err
	}
	password := string(passBuf)

	proxyUser, err := database.AuthenticateProxyUser(username, password)
	if err != nil {
		conn.Write([]byte{0x01, GeneralFailure})
		return nil, fmt.Errorf("invalid proxy credentials")
	}

	conn.Write([]byte{0x01, SuccessReply})

	return &AuthResult{
		UserID: proxyUser.ID,
		NodeID: proxyUser.NodeID,
	}, nil
}
