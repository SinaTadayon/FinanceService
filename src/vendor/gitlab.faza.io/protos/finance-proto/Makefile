build:
	protoc -I=. finance.proto --go_out=plugins=grpc:.
	protoc -I=. message.proto --go_out=plugins=grpc:.
	protoc -I=. data.proto --go_out=plugins=grpc:.
	ls *.pb.go | xargs -n1 -IX bash -c 'sed s/,omitempty// X > X.tmp && mv X{.tmp,}'