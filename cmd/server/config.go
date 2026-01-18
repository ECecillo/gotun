package main

type config struct {
	Port int `env:"SERVER_PORT" envDefault:"8080"`
}
