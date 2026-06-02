module github.com/ammiranda/otf_api/cmd/otf-cli

go 1.26

replace github.com/ammiranda/otf_api => ../../

require (
	github.com/AlecAivazis/survey/v2 v2.3.7
	github.com/ammiranda/otf_api v0.0.0-00010101000000-000000000000
	github.com/joho/godotenv v1.5.1
	github.com/mgutz/ansi v0.0.0-20200706080929-d51e80ef957d
	github.com/spf13/cobra v1.10.2
	golang.org/x/term v0.43.0
)

require (
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/kballard/go-shellquote v0.0.0-20180428030007-95032a82bc51 // indirect
	github.com/mattn/go-colorable v0.1.2 // indirect
	github.com/mattn/go-isatty v0.0.8 // indirect
	github.com/spf13/pflag v1.0.9 // indirect
	golang.org/x/sys v0.44.0 // indirect
	golang.org/x/text v0.4.0 // indirect
)
