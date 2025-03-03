package gomatrixserverlib

import (
	"encoding/json"

	"github.com/jchv/maidtrix/internal/matrixserver/spec"
)

// A Transaction is used to push data from one matrix server to another matrix
// server.
type Transaction struct {
	// The ID of the transaction.
	TransactionID TransactionID `json:"-"`
	// The server that sent the transaction.
	Origin spec.ServerName `json:"origin"`
	// The server that should receive the transaction.
	Destination spec.ServerName `json:"-"`
	// The millisecond posix timestamp on the origin server when the
	// transaction was created.
	OriginServerTS spec.Timestamp `json:"origin_server_ts"`
	// The IDs of the most recent transactions sent by the origin server to
	// the destination server. Multiple transactions can be sent by the origin
	// server to the destination server in parallel so there may be more than
	// one previous transaction.
	PreviousIDs []TransactionID `json:"-"`
	// The room events pushed from the origin server to the destination server
	// by this transaction. The events should either be events that originate
	// on the origin server or be join m.room.member events.
	PDUs []json.RawMessage `json:"pdus"`
	// The ephemeral events pushed from origin server to destination server
	// by this transaction. The events must orginate at the origin server.
	EDUs []EDU `json:"edus,omitempty"`
}

// A TransactionID identifies a transaction sent by a matrix server to another
// matrix server. The ID must be unique amongst the transactions sent from the
// origin server to the destination, but doesn't have to be globally unique.
// The ID must be safe to insert into a URL path segment. The ID should have a
// format matching '^[0-9A-Za-z\-_]*$'
type TransactionID string
