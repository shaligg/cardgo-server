package ws

import "time"

type HeartbeatConfig struct {
	Interval  time.Duration
	PongWait  time.Duration
	WriteWait time.Duration
}
