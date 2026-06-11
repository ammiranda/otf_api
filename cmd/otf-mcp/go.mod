module github.com/ammiranda/otf_api/cmd/otf-mcp

go 1.26

require github.com/ammiranda/otf_api v0.1.0

replace github.com/ammiranda/otf_api => ../../

require (
	github.com/danieljoos/wincred v1.2.3 // indirect
	github.com/godbus/dbus/v5 v5.2.2 // indirect
	github.com/zalando/go-keyring v0.2.8 // indirect
	golang.org/x/sys v0.27.0 // indirect
)
