package otf_api

import "github.com/zalando/go-keyring"

const keyringService = "otf-api"
const keyringUser = "config"

var keyringGet = func(service, user string) (string, error) {
	return keyring.Get(service, user)
}

var keyringSet = func(service, user, value string) error {
	return keyring.Set(service, user, value)
}


