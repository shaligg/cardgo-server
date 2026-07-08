package state

type Snapshot struct {
	UID     string
	Version int64
	Payload []byte
}
