import subprocess

class IPFS:
    # Current python libraries designed for IPFS access are 5+ versions behind in compatibility
    # so we simply subprocess the ipfs-go CLI here.

    @classmethod
    def add(self, path):
        # Invoke ipfs CLI, add file, and return its hash
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
    def fetch(self, hash_, dest):
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