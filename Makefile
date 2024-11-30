all : protobuf

protobuf : protobuf/api/cbdc.proto
	protoc -I ./protobuf/ \
	--go_out ./protobuf --go_out ./application-rbi --go_out ./application-hdfc --go_out ./application-axis --go_opt paths=source_relative \
	--go-grpc_out ./protobuf --go-grpc_out ./application-rbi  --go-grpc_out ./application-hdfc --go-grpc_out  ./application-axis --go-grpc_opt paths=source_relative \
  	--grpc-gateway_out ./protobuf --grpc-gateway_out ./application-rbi --grpc-gateway_out ./application-hdfc --grpc-gateway_out ./application-axis --grpc-gateway_opt paths=source_relative \
  	./protobuf/api/cbdc.proto