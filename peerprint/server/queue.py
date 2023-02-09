import sys
import os
import time
import copy
import tempfile
from pathlib import Path
from threading import Lock, Thread
from .proc import ServerProcess
import grpc
from .scripts.cert_gen import gen_certs
from multiprocessing import Condition
import peerprint.server.pkg.proto.state_pb2 as spb
import peerprint.server.pkg.proto.peers_pb2 as ppb
import peerprint.server.pkg.proto.command_pb2 as cpb
import peerprint.server.pkg.proto.command_pb2_grpc as command_grpc
from google.protobuf.any_pb2 import Any
from enum import Enum


# See scripts/cert_gen.py - must match common name of server cert
GRPC_OPT = (('grpc.ssl_target_name_override', 'peerprint-server'),)

class ChangeType(Enum):
    JOBS = 0
    PEERS = 1

class P2PTestServer():
    def __init__(self, opts, binary_path, logger):
        self._logger = logger
        self._opts = opts
        self._binary_path = binary_path
        self._proc = None

        # Using a temporary directory allows running multiple instances/queues
        # using the same filesystem (e.g. for development or containerized
        # farms)
        self._tmpdir = tempfile.TemporaryDirectory()
        if self._opts.cfg is None:
            self._opts.cfg = f"{self._tmpdir.name}/config.json"
        if self._opts.certsDir is None:
            self._opts.certsDir = f"{self._tmpdir.name}/certs/"
            os.mkdir(self._opts.certsDir)
            gen_certs(self._opts.certsDir, ncli=1)

        with open(Path(self._opts.certsDir)/ 'rootCA.crt', 'rb') as f:
            rootcert = f.read()
            assert(len(rootcert) > 0)
        with open(Path(self._opts.certsDir)/ 'domain.key', 'rb') as f:
            privkey = f.read()
            assert(len(privkey) > 0)
        with open(Path(self._opts.certsDir)/ 'domain.crt', 'rb') as f:
            certchain = f.read()
            assert(len(certchain) > 0)

        self.creds = grpc.ssl_channel_credentials(
                root_certificates=rootcert, 
                private_key=privkey, 
                certificate_chain=certchain)

        self._logger.debug("initializing server process")
        self._proc = ServerProcess(self._opts, self._binary_path, self._logger.getChild("proc"))
        

    def _call(self, method, v, timeout=15.0):
        with grpc.secure_channel(self._opts.addr, self.creds, options=GRPC_OPT) as channel:
            stub = command_grpc.CommandStub(channel)
            response = getattr(stub, method)(v, timeout=timeout)
            return response

    # ==== command service methods ====
    
    def ping(self):
        ret = srv._call("Ping", cpb.HealthCheck())
        return ret != None

    def get_id(self, network):
        rep = self._call("GetId", cpb.GetIDRequest(network=network))
        return rep.id

    def get_networks(self):
        rep = self._call("GetNetworks", cpb.GetNetworksRequest())
        return rep.networks

    def connect(self, **kwargs):
        # See pkg/proto/command.proto: message ConnectRequest
        self._call("Connect", cpb.ConnectRequest(**kwargs))

    def disconnect(self, network):
        # See pkg/proto/command.proto: message ConnectRequest
        self._call("Disconnect", cpb.DisconnectRequest(network=network))

    def set_record(self, network, **kwargs):
        # See pkg/proto/state.proto: message Record
        rank = spb.Rank(**kwargs['rank'])
        del kwargs['rank']
        rec = spb.Record(**kwargs, rank=rank)
        self._call("SetRecord", cpb.SetRecordRequest(network=network, record=rec))
    
    def set_completion(self, network, **kwargs):
        # See pkg/proto/state.proto: message Completion
        self._call("SetCompletion", cpb.SetCompletionRequest(network=network, completion=spb.Completion(**kwargs)))
    
    def crawl(self, network, batch_size=50, timeout_millis=20*1000, restart_crawl=False):
        # See pkg/proto/state.proto: message Completion
        rep = self._call("Crawl", cpb.CrawlRequest(network=network, batch_size=batch_size, restart_rawl=restart_crawl, timeout_millis=timeout_millis))

    def stream_events(self, network):
        with grpc.secure_channel(self._opts.addr, self.creds, options=GRPC_OPT) as channel:
            stub = command_grpc.CommandStub(channel)
            for msg in stub.StreamEvents(cpb.StreamEventsRequest(network=network)):
                yield msg 

    def get_records(self, network):
        with grpc.secure_channel(self._opts.addr, self.creds, options=GRPC_OPT) as channel:
            stub = command_grpc.CommandStub(channel)
            for msg in stub.StreamRecords(cpb.StreamRecordsRequest(network=network)):
                yield msg 

    def get_completions(self, network):
        with grpc.secure_channel(self._opts.addr, self.creds, options=GRPC_OPT) as channel:
            stub = command_grpc.CommandStub(channel)
            for msg in stub.StreamCompletions(cpb.StreamCompletionsRequest(network=network)):
                yield msg 

if __name__ == "__main__":
    import logging
    from .proc import ServerProcessOpts
    import sys

    assert(len(sys.argv) == 2)

    logging.basicConfig(level=logging.DEBUG)
    srv = P2PTestServer(ServerProcessOpts(
        addr="localhost:" + sys.argv[1],
        certsDir="certs/",
        serverCert="server.crt",
        serverKey="server.key",
        rootCert="rootCA.crt",
        ), "./server", logging.getLogger())
    input()
    print("Ping: ", srv.ping())
    print("Networks: ", srv.get_networks())
    NET = "testnet"
    srv.connect(
            network=NET, 
            addr="/ip4/0.0.0.0/tcp/0", 
            rendezvous="testrendy", 
            psk="12345", 
            local=True, 
            db_path=f"{srv._tmpdir.name}/testnet.sqlite3",
            privkey_path=f"{srv._tmpdir.name}/testk.priv", 
            pubkey_path=f"{srv._tmpdir.name}/testkey.pub",
            display_name="testsrv", 
            connect_timeout=20,
            sync_period=500,
            max_records_per_peer=20,
            max_tracked_peers=10
        )
    print("Networks: ", srv.get_networks())
    sid = srv.get_id(NET)
    print("ID: ", sid)

    evts = srv.stream_events(NET)
    def streamy():
        for e in evts:
            print("EVENT:", e.__repr__())
    Thread(target=streamy,daemon=True).start()
    print("Streaming thread started")

    i = 0
    while True:
        i += 1
        input()
        srv.set_record(NET, uuid=f"r{i}", approver=sid, tags=[], manifest="man", created=123, rank=dict(num=0, den=0, gen=0))
        print("Record submitted")

        input()
        print("Records:")
        for r in srv.get_records(NET):
            print(r)
        print("Completions:")
        for r in srv.get_completions(NET):
            print(r)


