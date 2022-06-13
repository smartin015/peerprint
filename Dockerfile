FROM python:3.7

ENV NODE_VERSION=16.13.0
ENV NVM_DIR=/root/.nvm
ENV PATH="${NVM_DIR}/versions/node/v${NODE_VERSION}/bin/:${PATH}"

RUN apt install -y curl && curl -o- https://raw.githubusercontent.com/nvm-sh/nvm/v0.39.0/install.sh | bash
RUN . "$NVM_DIR/nvm.sh" && nvm install ${NODE_VERSION} && nvm use v${NODE_VERSION} && nvm alias default v${NODE_VERSION}

# Brownie used for ethereum testing/building (has ganache as dependency)
RUN pip install eth-brownie && npm install -g ganache-cli

# Install peerprint dependencies
ADD . /peerprint
RUN cd /peerprint && python3 -m pip install .

# Install go-ipfs
RUN wget https://dist.ipfs.io/go-ipfs/v0.13.0/go-ipfs_v0.13.0_linux-amd64.tar.gz && tar -xvzf go-ipfs_v0.13.0_linux-amd64.tar.gz && cd go-ipfs && bash ./install.sh

WORKDIR /code

