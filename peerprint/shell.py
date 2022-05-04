import logging
import cmd
import traceback
import datetime
import json
from peewee import IntegrityError

from .storage import queries
from .adapter import Adapter, JobData, PeerData, PrinterData, SetData

class InMemoryAdapter(Adapter):
  def __init__(self, config, ns, addr):
    self.jobs = []
    self.ns = ns
    self.addr = addr
    self.config = config
    self.secondsUntilIdle = 0
    self.status = "OPERATIONAL"

  def get_namespace(self) -> str:
    return self.ns

  def get_host_addr(self) -> str:
    return self.addr

  def get_printer_config(self) -> PrinterData:
    return PrinterData(**self.config['printer'])

  def get_peer_state(self) -> PeerData:
    return PeerData(secondsUntilIdle=self.secondsUntilIdle, status=self.status)

  def get_jobs(self) -> list[JobData]:
    result = []
    for i,j in enumerate(self.jobs):
      jj = json.loads(j[1])
      result.append(JobData(
        hash_=j[0], local_id=i, name=jj['name'], 
        sets=[SetData(hash_=s['hash'], count=s['count'], 
        material_keys=s['material_keys']) for s in jj['sets']]))
    return result

  def upsert_job(self, hash_, json) -> int:
    self.jobs.append((hash_, json))
    return len(self.jobs)-1

  def remove_job(self, local_id):
    self.jobs = self.jobs[:local_id] + self.jobs[local_id+1:]
                                       

class Shell(cmd.Cmd):
    intro = 'Type help to list commands. Ctrl+C to exit\n'
    prompt = '>> '

    RESULT_TYPES = ("success", "failure", "cancelled")

    class OutputCapture:
      def __init__(self):
        self.out = ""
      def write(self, s):
        self.out += s
      def dump(self):
        s = self.out
        self.out = ""
        return s

    def attach(self, server, config):
      self.config = config
      self.server = server
      self.stdout = Shell.OutputCapture()
      self.use_rawinput = False

    def log(self, s):
      self.stdout.write(s + "\n")

    # ====== Network commands =====
 
    def do_join(self, arg):
      'Join a queue: ns'
      (ns, port) = arg.split(" ")
      addr = f"localhost:{port}"
      a = InMemoryAdapter(self.config, ns, addr)
      try: 
        self.server.join(a)
        self.log(f"joined {ns} ({addr})")
      except ValueError:
        self.log("Invalid argument")

    def do_leave(self, arg):
      'Leave a LAN queue: name'
      self.server.leave(arg)
      self.log(f"left {arg}")

    # ====== Job commands ======

    def do_create(self, arg):
      'Create a job: ns hash json'
      try:
        (ns, hash_, json) = arg.split()
        self.server.get(ns).createJob(hash_, json)
        self.log(f"Added job {hash_}")
      except IntegrityError:
        self.log(f"Job with hash {hash_} already exists in {ns}")
      except ValueError as e:
        self.log(f"ValueError: {e}")

    def do_assign(self, arg):
      'Assignments for ns: ns hash1:peer1 hash2:peer2 ....'
      aa = arg.split(' ')
      ns = aa[0]
      assignment = dict([s.split(':') for s in aa[1:]])
      self.server.get(ns).syncAssigned(assignment)        
      self.log(f"Assigned: {assignment}")
 
    def do_acquire(self, arg):
      'Claim the job: ns hash'
      try:
        (ns, hash_) = arg.split(' ')
        job = self.server.get(ns).acquireJob(hash_)
        self.log(f"Acquired job '{hash_}' (ns {ns})")
      except Exception:
        self.log(traceback.format_exc())

    def do_release(self, arg):
      'Release the job: ns hash'
      try:
        (ns, hash_) = arg.split(' ')
        job = self.server.get(ns).releaseJob(hash_)
        self.log(f"Released job '{hash_}' (ns {ns})")
      except LookupError as e:
        self.log(f"LookupError: {e}")
 
    def do_remove(self, arg):
      'Remove the job: ns hash'
      (ns, hash_) = arg.split(' ')
      try:
        local_id = self.server.get(ns).removeJob(hash_)
        self.log(f"Removed job '{hash_}' (ns {ns}, local_id {local_id})")
      except LookupError as e:
        self.log(f"LookupError: {e}")
 
    # ==== Metadata commands ====
    
    def do_syncpeer(self, arg):
      'Sync peer state: ns json'
      ns, data = arg.split(' ', 1)
      self.server.get(ns).syncPeer(json.loads(data))
      self.log(f"Synced peer state (ns {ns})")

    def do_syncfiles(self, arg):
      'Sync files: ns json'
      ns, data = arg.split(' ', 1)
      self.server.get(ns).registerFiles(json.loads(data))
      self.log(f"Synced peer state (ns {ns})")

    # ===== Database and inmemory (network queue) getters =====

    def do_files(self, arg):
      'Print list of known files and hashes of peer: peer ("local" for local files)'
      files = queries.getFiles(arg)
      self.log(f"=== {len(files)} File(s) from peer '{arg}' ===")
      for p, val in files.items():
        self.log(f"{p}\n\t{val}\n")

    def do_ns(self, arg):
      'Print current namespaces'
      self.log(str(list(self.server._pqs.keys())))

    def do_peers(self, arg):
      'Print peers in ns: ns'
      peers = queries.getPeers(arg)
      self.log(f"=== {len(peers)} Peer(s): ===")
      for p in peers:
        self.log('\n')
        self.log(f"{p.addr} ({p.model})")

    def do_jobs(self, arg):
      'List jobs in a namespace: ns'
      now = datetime.datetime.now()
      js = queries.getJobs(arg)
      self.log(f"=== {len(js)} Job(s) for namespace '{arg}' ===")
      for j in js:
        js = f"{j.hash_} (local_id={j.local_id})"
        if j.peerAssigned:
          js += f" assigned to {j.peerAssigned}"
          if j.peerLease is not None and j.peerLease > now:
            dt = (j.peerLease - now).total_seconds() / 60
            js += f" - leased for {dt:.1f} min"
        self.log(js)


def main():
    logging.basicConfig(level=logging.DEBUG)
    import sys  
    import yaml
    import argparse
    import zmq

    from .server import Server

    parser = argparse.ArgumentParser(description='Start a network queue server')
    parser.add_argument('--config', type=str, help='path to yaml config')
    args = parser.parse_args()

    with open(args.config, 'r') as f:
      data = yaml.safe_load(f.read())
    server = Server(data['db_path'], logging.getLogger("server"))

    sh = Shell()
    sh.attach(server, data)
    context = zmq.Context()
    socket = context.socket(zmq.REP)
    logging.info(f"Starting debug REP socket at {data['debug_socket']}")
    socket.bind(data['debug_socket'])
    while True:
      msg = socket.recv_string()
      logging.debug(f"USER: {msg}")
      sh.onecmd(msg)
      socket.send_string(sh.stdout.dump())

if __name__ == "__main__":
  main()
