package conf

import (
	"flag"

	"github.com/jseow5177/tcp-pool/internal/tcp"
)

type Config struct {
	HttpConfig *HttpConfig
	TcpConfig  *tcp.TcpConfig
}

type HttpConfig struct {
	Host string
	Port int
}

func InitConfig() *Config {
	c := &Config{
		HttpConfig: &HttpConfig{
			Host: *flag.String("http-host", "localhost", "host of http server"),
			Port: *flag.Int("http-port", 3030, "port of http server"),
		},
		TcpConfig: &tcp.TcpConfig{
			Host:         *flag.String("tcp-host", "localhost", "host of tcp server"),
			Port:         *flag.Int("tcp-port", 4000, "port of tcp server"),
			MaxIdleConns: *flag.Int("max-idle", 1, "max number of idle tcp conns"),
			MaxOpenConn:  *flag.Int("max-open", 0, "max number of open tcp conns"),
		},
	}

	flag.Parse()

	return c
}
