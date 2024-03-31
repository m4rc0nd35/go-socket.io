package socketio

// RedisAdapterConfig is configuration to create new adapter
type RedisAdapterConfig struct {
	Addr     string
	Prefix   string
	Network  string
	Password string
	DB       int
}

func (cfg *RedisAdapterConfig) getAddr() string {
	return cfg.Addr
}

func defaultConfig() *RedisAdapterConfig {
	return &RedisAdapterConfig{
		Addr:    "127.0.0.1:6379",
		Prefix:  "socket.io",
		Network: "tcp",
	}
}

func GetOptions(opts *RedisAdapterConfig) *RedisAdapterConfig {
	options := defaultConfig()

	if opts != nil {
		if opts.Addr != "" {
			options.Addr = opts.Addr
		}

		if opts.Prefix != "" {
			options.Prefix = opts.Prefix
		}

		if opts.Network != "" {
			options.Network = opts.Network
		}

		if opts.DB > 0 {
			options.DB = opts.DB
		}

		if len(opts.Password) > 0 {
			options.Password = opts.Password
		}
	}

	return options
}
