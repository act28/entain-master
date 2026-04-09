module git.neds.sh/matty/entain/sports

go 1.24.0

require (
	github.com/google/go-cmp v0.7.0
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.27.1
	github.com/mattn/go-sqlite3 v1.14.16
	google.golang.org/genproto/googleapis/api v0.0.0-20251202230838-ff82c1b0f217
	google.golang.org/grpc v1.79.3
	google.golang.org/grpc/cmd/protoc-gen-go-grpc v1.3.0
	google.golang.org/protobuf v1.36.10
	syreclabs.com/go/faker v1.2.3
)

require (
	github.com/kr/text v0.2.0 // indirect
	github.com/rogpeppe/go-internal v1.14.1 // indirect
	golang.org/x/net v0.48.0 // indirect
	golang.org/x/sys v0.39.0 // indirect
	golang.org/x/text v0.32.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20251202230838-ff82c1b0f217 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace syreclabs.com/go/faker => github.com/dmgk/faker v1.2.3
