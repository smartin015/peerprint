import subprocess
import logging
from .proc import IPFSDaemonProcess

proc = None

class IPFS:
    # Current python libraries designed for IPFS access are 5+ versions behind in compatibility
    # so we simply subprocess the ipfs-go CLI here.

    MAX_CUMULATIVE_SIZE = 10*1024*1024 # 10 MiB

    @classmethod
    def start_daemon(self):
        global proc
        if proc != None:
            return proc
        proc = IPFSDaemonProcess(logging.getLogger())
        return proc

    @classmethod 
    def stop_daemon(self):
        global proc
        if proc != None:
            proc.destroy()
            proc = None

    @classmethod
    def check(self):
        global proc
        if proc is None:
            raise Exception("IPFS daemon not started")
        if not proc.is_running():
            raise Exception("IPFS daemon not running")
        return

    @classmethod
    def add(self, path):
        IPFS.check()
        # Invoke ipfs CLI, add file, and return its hash
        # TODO check file size before adding so it doesn't get rejected out of hand
        result = subprocess.run(["ipfs", "add", "--cid-version=1", "-q", path], capture_output=True)
        if result.returncode != 0:
            raise Exception(f"add_file result {result.returncode}: {result.stderr}")
        return result.stdout.strip()

    @classmethod
    def pin(self, hash_):
        IPFS.check()
        result = subprocess.run(["ipfs", "pin", "add", hash_])
        return result.returncode == 0

    @classmethod
    def unpin(self, hash_):
        IPFS.check()
        result = subprocess.run(["ipfs", "pin", "rm", hash_])
        return result.returncode == 0

    @classmethod 
    def stat(self, hash_):
        IPFS.check()
        result = subprocess.run(["ipfs", "object", "stat", hash_], capture_output=True)
        if result.returncode != 0:
            raise Exception(f"stat result {result.returncode}: {result.stderr}")
        out = result.stdout.decode('utf8')
        result = dict()
        for line in out.split('\n'):
            if line.strip() == "":
                continue
            k, v = line.split(':', 1)
            result[k.strip()] = int(v)
        return result

    @classmethod
    def fetch(self, hash_, dest):
        IPFS.check()
        csz = self.stat(hash_)['CumulativeSize'] 
        if csz > self.MAX_CUMULATIVE_SIZE:
            raise Exception(f"IPFS: {hash_} cumulative size {csz} exceeds limit ({self.MAX_CUMULATIVE_SIZE})")

        result = subprocess.run(["ipfs", "get", f"--output={dest}", hash_])
        return result.returncode == 0

    @classmethod
    def fetchStr(self, hash_) -> str:
        IPFS.check()
        result = subprocess.run(["ipfs", "cat", hash_], capture_output=True)
        if result.returncode != 0:
            raise Exception(f"fetchStr result {result.returncode}: {result.stderr}")
        return result.stdout.decode('utf8').strip()


if __name__ == '__main__':
    result = IPFS.add("test.txt")
    print(result)
    print(IPFS.pin(result))
    print(IPFS.unpin(result))
    print(IPFS.fetch(result, "downloaded.txt"))
