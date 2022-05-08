import logging
import cmd
import traceback
import datetime
import json

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

    def attach(self, lan):
      self.lan = lan
      self.stdout = Shell.OutputCapture()
      self.use_rawinput = False

    def log(self, s):
      self.stdout.write(s + "\n")

    def do_create(self, arg):
      'Create a job: hash manifest '
      self.lan.q.createJob(*(arg.split(maxsplit=1)))
      self.log(f"Added job")

    def do_acquire(self, arg):
      'Claim the job: hash'
      job = self.lan.q.acquireJob(arg)
      self.log(f"Acquired job '{arg}'")

    def do_release(self, arg):
      'Release the job: hash'
      job = self.lan.q.releaseJob(arg)
      self.log(f"Released job '{arg}'")
 
    def do_remove(self, arg):
      'Remove the job: hash'
      local_id = self.lan.q.removeJob(arg)
      self.log(f"Removed job '{arg}'")
 
    def do_status(self, arg):
      self.lan.q.printStatus()

    # ==== Metadata commands ====
    
    def do_syncpeer(self, arg):
      'Sync peer state: json'
      self.lan.q.syncPeer(json.loads(arg))
      self.log(f"Synced peer state ({arg})")

    # ===== Database and inmemory (network queue) getters =====

    def do_peers(self, arg):
      'Print peers'
      for name, data in self.lan.q.peers.items():
          self.log(f"{name}: {data}")

    def do_jobs(self, arg):
      for name, data in self.lan.q.jobs.items():
          self.log(f"{name}: {data}")

    def do_locks(self, arg):
      for (lid, val) in self.lan.q.locks._ReplLockManager__lockImpl._ReplLockManagerImpl__locks.items():
          self.log(f"{lid}: {val}")


def main():
    logging.basicConfig(level=logging.DEBUG)
    import sys  
    import yaml
    import argparse
    import zmq

    from .lan_queue import LANPrintQueue
    parser = argparse.ArgumentParser(description='Start a network queue server')
    parser.add_argument('--ns', type=str)
    parser.add_argument('--addr', type=str)
    parser.add_argument('--debug', type=str)
    args = parser.parse_args()

    lan = LANPrintQueue(args.ns, args.addr, None, logging.getLogger("queue"))
    sh = Shell()
    sh.attach(lan)
    context = zmq.Context()
    socket = context.socket(zmq.REP)
    logging.info(f"Starting debug REP socket at {args.debug}")
    socket.bind(args.debug)
    while True:
      msg = socket.recv_string()
      logging.debug(f"USER: {msg}")
      sh.onecmd(msg)
      socket.send_string(sh.stdout.dump())

if __name__ == "__main__":
  main()
