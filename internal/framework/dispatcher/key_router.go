package dispatcher

import "hash/fnv"

type Domain string

const (
	DomainPlayer  Domain = "player"
	DomainGuild   Domain = "guild"
	DomainChannel Domain = "channel"
)

func RouteShard(domain Domain, key string, shards int) int {
	if shards <= 0 {
		return 0
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(string(domain) + ":" + key))
	return int(h.Sum32()) % shards
}
