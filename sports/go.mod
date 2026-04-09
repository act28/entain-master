module git.neds.sh/matty/entain/sports

go 1.16

require (
	github.com/google/go-cmp v0.5.4
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.3.0
	github.com/mattn/go-sqlite3 v1.14.6
	golang.org/x/net v0.0.0-20210226172049-e18ecbb05110 // indirect
	google.golang.org/genproto v0.0.0-20210226172003-ab064af71705
	google.golang.org/grpc v1.36.0
	google.golang.org/grpc/cmd/protoc-gen-go-grpc v1.1.0
	google.golang.org/protobuf v1.25.1-0.20201208041424-160c7477e0e8
	syreclabs.com/go/faker v1.2.3
)

replace syreclabs.com/go/faker => github.com/dmgk/faker v1.2.3
