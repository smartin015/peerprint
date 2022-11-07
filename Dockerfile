FROM golang:1.19

FROM python:3.7
ENV PROTOC_ZIP=protoc-21.9-linux-x86_64.zip

RUN pip3 install --upgrade protobuf pyzmq
RUN apt-get update && apt-get -y install --no-install-recommends libczmq-dev libzmq5 unzip curl && rm -rf /var/lib/apt/lists/*

RUN wget https://dist.ipfs.tech/kubo/v0.16.0/kubo_v0.16.0_linux-amd64.tar.gz && tar -xvzf kubo_v0.16.0_linux-amd64.tar.gz \
 && cd kubo && ./install.sh && ipfs --version

COPY --from=0 /usr/local/go/bin/go /bin/go
COPY --from=0 /usr/local/go /usr/local/go
RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@latest \
 && echo 'export PATH="$PATH:$(go env GOPATH)/bin"' >> ~/.bashrc 

RUN curl -OL https://github.com/protocolbuffers/protobuf/releases/download/v21.9/$PROTOC_ZIP \
    && unzip -o $PROTOC_ZIP -d /usr/local bin/protoc \
    && unzip -o $PROTOC_ZIP -d /usr/local 'include/*' \ 
    && rm -f $PROTOC_ZIP

ADD . /peerprint

RUN cd /peerprint && python3 -m pip install .
