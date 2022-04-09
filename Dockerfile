FROM python:3.9

RUN apt-get update && apt-get -y install --no-install-recommends libzmq3-dev && rm -rf /var/lib/apt/lists/*

ADD . /peerprint

RUN cd /peerprint && python3 -m pip install .
