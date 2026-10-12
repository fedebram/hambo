package cliconfig

type Config struct {
	ServerURL  string `yaml:"server"`
	CACert     string `yaml:"ca-cert"`
	ClientCert string `yaml:"client-cert"`
	ClientKey  string `yaml:"client-key"`
}
