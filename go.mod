module github.com/numinance/numinance-spec

go 1.27

toolchain go1.27.3

require (
	connectrpc.com/connect v1.21.1
	go.yaml.in/yaml/v3 v3.0.5
	google.golang.org/grpc v1.84.0
	google.golang.org/protobuf v1.36.12
)

require (
	golang.org/x/net v0.60.0 // indirect; floor for GO-2026-6603, GO-2026-6611, GO-2026-6612, GO-2026-6617 (govulncheck)
	golang.org/x/sys v0.48.0 // indirect
	golang.org/x/text v0.42.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260706201446-f0a921348800 // indirect
)
