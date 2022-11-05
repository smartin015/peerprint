FROM golang:1.19

FROM python:3.7

RUN pip3 install --upgrade protobuf pyzmq
RUN apt-get update && apt-get -y install --no-install-recommends libczmq-dev libzmq5 && rm -rf /var/lib/apt/lists/*

RUN wget https://dist.ipfs.tech/kubo/v0.16.0/kubo_v0.16.0_linux-amd64.tar.gz && tar -xvzf kubo_v0.16.0_linux-amd64.tar.gz \
 && cd kubo && ./install.sh && ipfs --version

COPY --from=0 /usr/local/go/bin/go /bin/go
COPY --from=0 /usr/local/go /usr/local/go
RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@latest

ADD . /peerprint

RUN cd /peerprint && python3 -m pip install .
