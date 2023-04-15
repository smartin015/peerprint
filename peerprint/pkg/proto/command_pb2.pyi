import state_pb2 as _state_pb2
import peers_pb2 as _peers_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from typing import ClassVar as _ClassVar, Iterable as _Iterable, Mapping as _Mapping, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class AdvertiseRequest(_message.Message):
    __slots__ = ["config", "local"]
    CONFIG_FIELD_NUMBER: _ClassVar[int]
    LOCAL_FIELD_NUMBER: _ClassVar[int]
    config: _peers_pb2.NetworkConfig
    local: bool
    def __init__(self, local: bool = ..., config: _Optional[_Union[_peers_pb2.NetworkConfig, _Mapping]] = ...) -> None: ...

class ConnectRequest(_message.Message):
    __slots__ = ["addr", "connect_timeout", "db_path", "display_name", "extra_bootstrap_peers", "extra_relay_peers", "local", "max_records_per_peer", "max_tracked_peers", "network", "privkey_path", "psk", "pubkey_path", "rendezvous", "sync_period"]
    ADDR_FIELD_NUMBER: _ClassVar[int]
    CONNECT_TIMEOUT_FIELD_NUMBER: _ClassVar[int]
    DB_PATH_FIELD_NUMBER: _ClassVar[int]
    DISPLAY_NAME_FIELD_NUMBER: _ClassVar[int]
    EXTRA_BOOTSTRAP_PEERS_FIELD_NUMBER: _ClassVar[int]
    EXTRA_RELAY_PEERS_FIELD_NUMBER: _ClassVar[int]
    LOCAL_FIELD_NUMBER: _ClassVar[int]
    MAX_RECORDS_PER_PEER_FIELD_NUMBER: _ClassVar[int]
    MAX_TRACKED_PEERS_FIELD_NUMBER: _ClassVar[int]
    NETWORK_FIELD_NUMBER: _ClassVar[int]
    PRIVKEY_PATH_FIELD_NUMBER: _ClassVar[int]
    PSK_FIELD_NUMBER: _ClassVar[int]
    PUBKEY_PATH_FIELD_NUMBER: _ClassVar[int]
    RENDEZVOUS_FIELD_NUMBER: _ClassVar[int]
    SYNC_PERIOD_FIELD_NUMBER: _ClassVar[int]
    addr: str
    connect_timeout: int
    db_path: str
    display_name: str
    extra_bootstrap_peers: _containers.RepeatedScalarFieldContainer[str]
    extra_relay_peers: _containers.RepeatedScalarFieldContainer[str]
    local: bool
    max_records_per_peer: int
    max_tracked_peers: int
    network: str
    privkey_path: str
    psk: str
    pubkey_path: str
    rendezvous: str
    sync_period: int
    def __init__(self, network: _Optional[str] = ..., addr: _Optional[str] = ..., rendezvous: _Optional[str] = ..., psk: _Optional[str] = ..., local: bool = ..., db_path: _Optional[str] = ..., privkey_path: _Optional[str] = ..., pubkey_path: _Optional[str] = ..., display_name: _Optional[str] = ..., connect_timeout: _Optional[int] = ..., sync_period: _Optional[int] = ..., max_records_per_peer: _Optional[int] = ..., max_tracked_peers: _Optional[int] = ..., extra_bootstrap_peers: _Optional[_Iterable[str]] = ..., extra_relay_peers: _Optional[_Iterable[str]] = ...) -> None: ...

class CrawlRequest(_message.Message):
    __slots__ = ["batch_size", "network", "restart_crawl", "timeout_millis"]
    BATCH_SIZE_FIELD_NUMBER: _ClassVar[int]
    NETWORK_FIELD_NUMBER: _ClassVar[int]
    RESTART_CRAWL_FIELD_NUMBER: _ClassVar[int]
    TIMEOUT_MILLIS_FIELD_NUMBER: _ClassVar[int]
    batch_size: int
    network: str
    restart_crawl: bool
    timeout_millis: int
    def __init__(self, network: _Optional[str] = ..., restart_crawl: bool = ..., batch_size: _Optional[int] = ..., timeout_millis: _Optional[int] = ...) -> None: ...

class CrawlResult(_message.Message):
    __slots__ = ["errors", "network", "remaining"]
    ERRORS_FIELD_NUMBER: _ClassVar[int]
    NETWORK_FIELD_NUMBER: _ClassVar[int]
    REMAINING_FIELD_NUMBER: _ClassVar[int]
    errors: _containers.RepeatedScalarFieldContainer[str]
    network: str
    remaining: int
    def __init__(self, network: _Optional[str] = ..., remaining: _Optional[int] = ..., errors: _Optional[_Iterable[str]] = ...) -> None: ...

class DisconnectRequest(_message.Message):
    __slots__ = ["network"]
    NETWORK_FIELD_NUMBER: _ClassVar[int]
    network: str
    def __init__(self, network: _Optional[str] = ...) -> None: ...

class Event(_message.Message):
    __slots__ = ["name", "progress"]
    NAME_FIELD_NUMBER: _ClassVar[int]
    PROGRESS_FIELD_NUMBER: _ClassVar[int]
    name: str
    progress: Progress
    def __init__(self, name: _Optional[str] = ..., progress: _Optional[_Union[Progress, _Mapping]] = ...) -> None: ...

class GetConnectionsRequest(_message.Message):
    __slots__ = []
    def __init__(self) -> None: ...

class GetConnectionsResponse(_message.Message):
    __slots__ = ["networks"]
    NETWORKS_FIELD_NUMBER: _ClassVar[int]
    networks: _containers.RepeatedCompositeFieldContainer[ConnectRequest]
    def __init__(self, networks: _Optional[_Iterable[_Union[ConnectRequest, _Mapping]]] = ...) -> None: ...

class GetIDRequest(_message.Message):
    __slots__ = ["network"]
    NETWORK_FIELD_NUMBER: _ClassVar[int]
    network: str
    def __init__(self, network: _Optional[str] = ...) -> None: ...

class GetIDResponse(_message.Message):
    __slots__ = ["id"]
    ID_FIELD_NUMBER: _ClassVar[int]
    id: str
    def __init__(self, id: _Optional[str] = ...) -> None: ...

class HealthCheck(_message.Message):
    __slots__ = []
    def __init__(self) -> None: ...

class Ok(_message.Message):
    __slots__ = []
    def __init__(self) -> None: ...

class Progress(_message.Message):
    __slots__ = ["completed", "resolved_completer", "uuid"]
    COMPLETED_FIELD_NUMBER: _ClassVar[int]
    RESOLVED_COMPLETER_FIELD_NUMBER: _ClassVar[int]
    UUID_FIELD_NUMBER: _ClassVar[int]
    completed: bool
    resolved_completer: str
    uuid: str
    def __init__(self, uuid: _Optional[str] = ..., resolved_completer: _Optional[str] = ..., completed: bool = ...) -> None: ...

class SetCompletionRequest(_message.Message):
    __slots__ = ["completion", "network"]
    COMPLETION_FIELD_NUMBER: _ClassVar[int]
    NETWORK_FIELD_NUMBER: _ClassVar[int]
    completion: _state_pb2.Completion
    network: str
    def __init__(self, network: _Optional[str] = ..., completion: _Optional[_Union[_state_pb2.Completion, _Mapping]] = ...) -> None: ...

class SetRecordRequest(_message.Message):
    __slots__ = ["network", "record"]
    NETWORK_FIELD_NUMBER: _ClassVar[int]
    RECORD_FIELD_NUMBER: _ClassVar[int]
    network: str
    record: _state_pb2.Record
    def __init__(self, network: _Optional[str] = ..., record: _Optional[_Union[_state_pb2.Record, _Mapping]] = ...) -> None: ...

class SetStatusRequest(_message.Message):
    __slots__ = ["network", "status"]
    NETWORK_FIELD_NUMBER: _ClassVar[int]
    STATUS_FIELD_NUMBER: _ClassVar[int]
    network: str
    status: _peers_pb2.ClientStatus
    def __init__(self, network: _Optional[str] = ..., status: _Optional[_Union[_peers_pb2.ClientStatus, _Mapping]] = ...) -> None: ...

class StopAdvertisingRequest(_message.Message):
    __slots__ = ["local", "uuid"]
    LOCAL_FIELD_NUMBER: _ClassVar[int]
    UUID_FIELD_NUMBER: _ClassVar[int]
    local: bool
    uuid: str
    def __init__(self, local: bool = ..., uuid: _Optional[str] = ...) -> None: ...

class StreamAdvertisementsRequest(_message.Message):
    __slots__ = ["local"]
    LOCAL_FIELD_NUMBER: _ClassVar[int]
    local: bool
    def __init__(self, local: bool = ...) -> None: ...

class StreamCompletionsRequest(_message.Message):
    __slots__ = ["network", "uuid"]
    NETWORK_FIELD_NUMBER: _ClassVar[int]
    UUID_FIELD_NUMBER: _ClassVar[int]
    network: str
    uuid: str
    def __init__(self, network: _Optional[str] = ..., uuid: _Optional[str] = ...) -> None: ...

class StreamEventsRequest(_message.Message):
    __slots__ = ["network"]
    NETWORK_FIELD_NUMBER: _ClassVar[int]
    network: str
    def __init__(self, network: _Optional[str] = ...) -> None: ...

class StreamNetworksRequest(_message.Message):
    __slots__ = ["local"]
    LOCAL_FIELD_NUMBER: _ClassVar[int]
    local: bool
    def __init__(self, local: bool = ...) -> None: ...

class StreamPeersRequest(_message.Message):
    __slots__ = ["network"]
    NETWORK_FIELD_NUMBER: _ClassVar[int]
    network: str
    def __init__(self, network: _Optional[str] = ...) -> None: ...

class StreamRecordsRequest(_message.Message):
    __slots__ = ["network", "uuid"]
    NETWORK_FIELD_NUMBER: _ClassVar[int]
    UUID_FIELD_NUMBER: _ClassVar[int]
    network: str
    uuid: str
    def __init__(self, network: _Optional[str] = ..., uuid: _Optional[str] = ...) -> None: ...

class SyncLobbyRequest(_message.Message):
    __slots__ = []
    def __init__(self) -> None: ...
