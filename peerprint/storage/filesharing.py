import hashlib
import tempfile
import zipfile
import json
from pathlib import Path

try:
    import zlib
    compression = zipfile.ZIP_DEFLATED
except:
    compression = zipfile.ZIP_STORED

# https://stackoverflow.com/a/3431838
def _content_md5(path) -> str:
    hash_md5 = hashlib.md5()
    with open(path, "rb") as f:
        for chunk in iter(lambda: f.read(4096), b""):
            hash_md5.update(chunk)
    return hash_md5.hexdigest()


def pack_job(manifest, filepaths):
    # TODO validation - ensure the correct files are available given the manifest object)
    f = tempfile.NamedTemporaryFile(suffix=".zip")
    zf = zipfile.ZipFile(f.name, mode='w')
    try:
        for path in filepaths:
            print('adding', path)
            zf.write(path, compress_type=compression)
        zf.writestr("manifest.json", json.dumps(manifest))
    finally:
        zf.close()
    
    f.job_hash = _content_md5(f.name)
    # Note: it's the responsibility of the caller to call f.cleanup() when done with the file.
    return f


def unpack_job(path, outdir):
    with zipfile.ZipFile(path, mode='r') as zf:
        zf.extractall(path=outdir)
        with open(Path(outdir) / 'manifest.json', 'r') as f:
            manifest = json.loads(f.read())
        return manifest, [n for n in zf.namelist() if Path(n).name != 'manifest.json']


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
