# export PATH="$PATH:$(go env GOPATH)/bin"
protoc --go_out=./pkg/helper/events --go_opt=paths=source_relative --go-grpc_out=./pkg/helper/events --go-grpc_opt=paths=source_relative events.proto