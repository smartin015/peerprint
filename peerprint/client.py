import grpc
from pathlib import Path
import time
from enum import IntEnum
from google.protobuf.json_format import MessageToDict
import peerprint.pkg.proto.command_pb2_grpc as command_grpc
import peerprint.pkg.proto.state_pb2 as spb
import peerprint.pkg.proto.peers_pb2 as ppb
import peerprint.pkg.proto.command_pb2 as cpb

RpcError = grpc.RpcError

# See scripts/cert_gen.py - must match common name of server cert
GRPC_OPT = (('grpc.ssl_target_name_override', 'peerprint-server'),)

class P2PClient():
    class CompletionType(IntEnum):
        UNKNOWN = spb.UNKNOWN_COMPLETION_TYPE
        ACQUIRE = spb.ACQUIRE
        RELEASE = spb.RELEASE
        TOMBSTONE = spb.TOMBSTONE

    def __init__(self, addr, certsDir, logger):
        self._addr = addr
        self._logger = logger

        rootPath = Path(certsDir)/ 'rootCA.crt'
        keyPath = Path(certsDir)/ 'client1.key'
        certPath = Path(certsDir)/ 'client1.crt'
        spins = 0
        while not rootPath.exists() or not keyPath.exists() or not certPath.exists():
            self._logger.info("Waiting for certificate files to exist...")
            time.sleep(5.0)
            spins += 1
            if spins >= 6:
                raise Exception("Timed out waiting for certificate files")

        with open(rootPath, 'rb') as f:
            rootcert = f.read()
            assert(len(rootcert) > 0)
        with open(keyPath, 'rb') as f:
            privkey = f.read()
            assert(len(privkey) > 0)
        with open(certPath, 'rb') as f:
            certchain = f.read()
            assert(len(certchain) > 0)

        self.creds = grpc.ssl_channel_credentials(
                root_certificates=rootcert, 
                private_key=privkey, 
                certificate_chain=certchain)

    def _call(self, method, v, timeout=15.0):
        with grpc.secure_channel(self._addr, self.creds, options=GRPC_OPT) as channel:
            stub = command_grpc.CommandStub(channel)
            response = getattr(stub, method)(v, timeout=timeout)
            return response

    def _stream(self, method, req):
        with grpc.secure_channel(self._addr, self.creds, options=GRPC_OPT) as channel:
            stub = command_grpc.CommandStub(channel)
            for msg in getattr(stub, method)(req):
                yield msg 

    def is_ready(self):
        return self.ping()

    # ==== command service methods ====
    
    def ping(self):
        try:
            ret = self._call("Ping", cpb.HealthCheck())
            return ret != None
        except grpc.RpcError:
            return False

    def get_id(self, network):
        rep = self._call("GetId", cpb.GetIDRequest(network=network))
        return rep.id

    def get_connections(self):
        rep = self._call("GetConnections", cpb.GetConnectionsRequest())
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
    
    def set_status(self, network, **kwargs):
        # See pkg/proto/peers.proto: message PeerStatus
        self._call("SetStatus", cpb.SetStatusRequest(network=network, status=ppb.ClientStatus(**kwargs)))

    def crawl(self, network, batch_size=50, timeout_millis=20*1000, restart_crawl=False):
        # See pkg/proto/state.proto: message Completion
        rep = self._call("Crawl", cpb.CrawlRequest(network=network, batch_size=batch_size, restart_rawl=restart_crawl, timeout_millis=timeout_millis))

    def stream_events(self, network):
        try:
            for v in self._stream("StreamEvents", cpb.StreamEventsRequest(network=network)):
                yield v
        except grpc._channel._MultiThreadedRendezvous as e:
            # Socket closed error occurs on Ctrl+C; shorten the exception
            # into a simple debug log so it doesn't clog up the console
            if e.code() == grpc.StatusCode.UNAVAILABLE and 'Socket closed' in e.details():
                self._logger.debug("Event stream socket closed")
            else:
                raise

    def get_networks(self):
        for v in self._stream("StreamNetworks", cpb.StreamNetworksRequest()):
            yield v

    def get_peers(self, network):
        for v in self._stream("StreamPeers", cpb.StreamPeersRequest(network=network)):
            yield MessageToDict(v) # Convert to dict here as it's almost always json serialized

    def get_records(self, network, uuid=None):
        for v in self._stream("StreamRecords", cpb.StreamRecordsRequest(network=network, uuid=uuid)):
            yield v

    def get_completions(self, network, uuid=None):
        for v in self._stream("StreamCompletions", cpb.StreamCompletionsRequest(network=network, uuid=uuid)):
            yield v

    def get_advertisements(self, local):
        for v in self._stream("StreamAdvertisements", cpb.StreamAdvertisementsRequest(local=local)):
            yield v

if __name__ == "__main__":
    import logging
    import sys

    assert(len(sys.argv) == 2)

    logging.basicConfig(level=logging.DEBUG)
    srv = P2PServer(P2PServerOpts(
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

