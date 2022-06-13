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
      self.lan.q.setJob(*(arg.split(maxsplit=1)))
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
      'Sync peer state: status run'
      status, runstr = arg.split(maxsplit=1)
      self.lan.q.syncPeer(status, json.loads(runstr))
      self.log(f"Synced peer state ({arg})")

    # ===== Database and inmemory (network queue) getters =====

    def do_peers(self, arg):
      'Print peers'
      for name, data in self.lan.q.getPeers().items():
          self.log(f"{name}: {data}")

    def do_jobs(self, arg):
      for job in self.lan.q.getJobs():
          self.log(str(job))

    def do_locks(self, arg):
      for (peer, locks) in self.lan.q.locks.getPeerLocks().items():
          self.log(f"{peer}: {locks}")


def main():
    logging.basicConfig(level=logging.DEBUG)
    import sys  
    import argparse

    from .lan_queue import LANPrintQueue
    parser = argparse.ArgumentParser(description='Start a network queue server')
    parser.add_argument('--ns', type=str)
    parser.add_argument('--addr', type=str)
    args = parser.parse_args()

    def on_update(q):
        print("<update>")

    lan = LANPrintQueue(args.ns, args.addr, None, on_update, logging.getLogger("queue"))
    sh = Shell()
    sh.attach(lan)
    while True:
      msg = input(">> ")
      sh.onecmd(msg)
      sys.stdout.write(sh.stdout.dump())

if __name__ == "__main__":
  main()
