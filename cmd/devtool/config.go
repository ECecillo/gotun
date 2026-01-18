package main

import "time"

type config struct {
	Host     string        `env:"DEVTOOL_HOST" envDefault:"10.0.0.1"`
	Port     int           `env:"DEVTOOL_PORT" envDefault:"8080"`
	Message  string        `env:"DEVTOOL_MSG" envDefault:"hello from devtool"`
	Timeout  time.Duration `env:"DEVTOOL_TIMEOUT" envDefault:"2s"`
	Protocol string        `env:"DEVTOOL_PROTOCOL" envDefault:"udp"`
}
