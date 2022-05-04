from dataclasses import dataclass 

@dataclass
class SetData():
  hash_: str
  count: int
  material_keys: list[str]

@dataclass 
class JobData():
  local_id: int
  name: str
  sets: list[SetData]

@dataclass
class PeerData():
  status: str
  secondsUntilIdle: int

@dataclass
class PrinterData():
  width: float
  height: float
  depth: float

class Adapter:
  def get_namespace(self) -> str:
    raise NotImplementedError
  def get_host_addr(self) -> str:
    raise NotImplementedError
  def get_printer_config(self) -> PrinterData:
    raise NotImplementedError
  def get_peer_state(self) -> PeerData:
    raise NotImplementedError
  def get_jobs(self) -> JobData:
    raise NotImplementedError
  def on_ready(self): 
    pass # Optional
  def upsert_job(self, json) -> int:
    raise NotImplementedError
  def remove_job(self, local_id):
    raise NotImplementedError
