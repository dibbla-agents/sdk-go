package communication

import "errors"

var (
	// ErrNotConnected indicates the communicator is not connected
	ErrNotConnected = errors.New("not connected to workflow server")

	// ErrChannelFull indicates the communication channel is full
	ErrChannelFull = errors.New("communication channel is full")

	// ErrInvalidMode indicates an invalid communication mode was specified
	ErrInvalidMode = errors.New("invalid communication mode")
)

