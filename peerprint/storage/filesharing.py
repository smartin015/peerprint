import hashlib

# https://stackoverflow.com/a/3431838
def _content_md5(path) -> str:
    hash_md5 = hashlib.md5()
    with open(path, "rb") as f:
        for chunk in iter(lambda: f.read(4096), b""):
            hash_md5.update(chunk)
    return hash_md5.hexdigest()

def analyze(paths):
  hashes = {}
  for path in paths:
    hashes[path] = _content_md5(path)
  return hashes
    
def downloadFile(self, url:str, dest:str):
    # Get the equivalent path on disk
    written = 0
    self._logger.debug(f"Opening URL {url}")
    with requests.get(url, stream=True) as r:
      r.raise_for_status()
      with open(dest, 'wb') as f:
          for chunk in r.iter_content(chunk_size=8192):
              f.write(chunk)
              written += len(chunk)
    self._logger.debug(f"Wrote {written} bytes to {dest}")
    return dest
