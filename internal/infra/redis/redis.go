package redis

type Config struct {
	Addr string
}

type Client struct{}

func New(cfg Config) (*Client, error) {
	_ = cfg
	return &Client{}, nil
}
