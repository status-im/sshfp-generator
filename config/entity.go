package config

type Config struct {
	ConsulToken     string `json:"consulKey"`
	CloudflareToken string `json:"cloudflareKey"`
	DomainName      string `json:"domain"`
}
