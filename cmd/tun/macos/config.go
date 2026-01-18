//go:build darwin

package main

type config struct {
	SourceIP    string `env:"TUN_SOURCE_IP" envDefault:"10.0.0.2"`
	DestIP      string `env:"TUN_DEST_IP" envDefault:"10.0.0.1"`
	ForwardHost string `env:"TUN_FORWARD_HOST"`
	ForwardPort int    `env:"TUN_FORWARD_PORT" envDefault:"8080"`
}
