package constant

import (
	"net"

	N "github.com/qauzy/mat/common/net"

	"github.com/gofrs/uuid/v5"
)

type PlainContext interface {
	ID() uuid.UUID
}

type ConnContext interface {
	PlainContext
	Metadata() *Metadata
	Conn() *N.BufferedConn
}

type PacketConnContext interface {
	PlainContext
	Metadata() *Metadata
	PacketConn() net.PacketConn
}
