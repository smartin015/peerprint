import subprocess

class IPFS:
    # Current python libraries designed for IPFS access are 5+ versions behind in compatibility
    # so we simply subprocess the ipfs-go CLI here.

    MAX_CUMULATIVE_SIZE = 10*1024*1024 # 10 MiB

    @classmethod
    def add(self, path):
        # Invoke ipfs CLI, add file, and return its hash
        # TODO check file size before adding so it doesn't get rejected out of hand
        result = subprocess.run(["ipfs", "add", "--cid-version=1", "-q", path], capture_output=True)
        if result.returncode != 0:
            raise Exception(f"add_file result {result.returncode}: {result.stderr}")
        return result.stdout.strip()

    @classmethod
    def pin(self, hash_):
        result = subprocess.run(["ipfs", "pin", "add", hash_])
        return result.returncode == 0

    @classmethod
    def unpin(self, hash_):
        result = subprocess.run(["ipfs", "pin", "rm", hash_])
        return result.returncode == 0

    @classmethod 
    def stat(self, hash_):
        result = subprocess.run(["ipfs", "object", "stat", hash_], capture_output=True)
        if result.returncode != 0:
            raise Exception(f"stat result {result.returncode}: {result.stderr}")
        return dict([(k, int(v)) for line in result.stdout.split('\n') for k,v in line.split(':')])

    @classmethod
    def fetch(self, hash_, dest):
        csz = self.stat(hash_)['CumulativeSize'] 
        if csz > self.MAX_CUMULATIVE_SIZE:
            raise Exception(f"IPFS: {hash_} cumulative size {csz} exceeds limit ({self.MAX_CUMULATIVE_SIZE})")

        result = subprocess.run(["ipfs", "get", f"--output={dest}", hash_])
        return result.returncode == 0

    @classmethod
    def fetchStr(self, hash_) -> str:
        result = subprocess.run(["ipfs", "cat", hash_], capture_output=True)
        if result.returncode != 0:
            raise Exception(f"fetchStr result {result.returncode}: {result.stderr}")
        return result.stdout.strip()


if __name__ == '__main__':
    result = IPFS.add("test.txt")
    print(result)
    print(IPFS.pin(result))
    print(IPFS.unpin(result))
    print(IPFS.fetch(result, "downloaded.txt"))
