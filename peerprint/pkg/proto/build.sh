#!/bin/bash

python3 -m grpc_tools.protoc -I. --python_out=. --pyi_out=. --grpc_python_out=. command.proto
protoc --go_out=. --python_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative  *.proto

sed -i 's/import command_pb2/import peerprint.pkg.proto.command_pb2/g' ./*.py
sed -i 's/import state_pb2/import peerprint.pkg.proto.state_pb2/g' ./*.py
sed -i 's/import peers_pb2/import peerprint.pkg.proto.peers_pb2/g' ./*.py

