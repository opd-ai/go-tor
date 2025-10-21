module github.com/opd-ai/go-tor/examples/bine-examples/all-in-one

go 1.24.9

replace github.com/opd-ai/go-tor => ../../..

require (
	github.com/cretz/bine v0.2.0
	github.com/opd-ai/go-tor v0.0.0-00010101000000-000000000000
	golang.org/x/net v0.45.0
)

require (
	golang.org/x/crypto v0.43.0 // indirect
	golang.org/x/sys v0.37.0 // indirect
)
