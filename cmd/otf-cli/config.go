package main

import "github.com/ammiranda/otf_api/otf_api"

type IPLocation struct {
	Lat     float64 `json:"lat"`
	Lon     float64 `json:"lon"`
	City    string  `json:"city"`
	Region  string  `json:"regionName"`
	Country string  `json:"country"`
}

func loadConfig() (otf_api.CLIConfig, error) {
	return otf_api.LoadConfig()
}

func saveConfig(config otf_api.CLIConfig) error {
	return otf_api.SaveConfig(config)
}
