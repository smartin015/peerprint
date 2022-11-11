#!/bin/bash

echo "Compiling .proto files"
protoc --python-out=. --go_out=. --go_opt=paths=source_relative proto/*.proto
sed -i 's/from proto/from peerprint.server.proto/g' proto/state_pb2.py

echo "Building peerprint_server binary"
go build .


