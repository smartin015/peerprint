#!/bin/bash

protoc proto/rpc.proto -I. --go_out=:.
