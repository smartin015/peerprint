import hashlib
import zipfile
import json
import time
import re
from pathlib import Path
from peerprint import __version__ as version

try:
    import zlib
    compression = zipfile.ZIP_DEFLATED
except:
    compression = zipfile.ZIP_STORED

# https://stackoverflow.com/a/3431838
def _content_hash(path) -> str:
    h = hashlib.sha256()
    with open(path, "rb") as f:
        for chunk in iter(lambda: f.read(4096), b""):
            h.update(chunk)
    return h.hexdigest()


# Inspired by https://stackoverflow.com/a/1007615
def packed_name(s, ts=time.time()):
    if s.strip() == "":
        s = "untitled"

    # Remove all non-word characters (everything except numbers and letters)
    s = re.sub(r"[^\w\s]", '', s)
    # Replace all runs of whitespace with underscore
    s = re.sub(r"\s+", '_', s)

    return f"cpq_{s}_{int(ts)}.gjob"


def pack_job(manifest: dict, filepaths: dict, dest: str):
    # TODO validation - ensure the correct files are available given the manifest object)
    zf = zipfile.ZipFile(dest, mode='w')

    # Sanitize short paths
    filepaths = dict([(short.split("/")[-1], full) for (short, full) in filepaths.items()])

    # Strip off paths in manifest (paths are stripped in zip file as well)
    # and remove unnecessary state/identity fields
    for s in manifest["sets"]:
        s["path"] = s["path"].split("/")[-1]
        if filepaths.get(s["path"]) is None:
            raise ValueError(f"Job contains set with path={s['path']}, but filepaths has no matching short name")
        for k in ("id", "remaining", "rank", "sd"):
            s.pop(k, None)
    for k in ("acquired", "draft", "id", "remaining", "created"):
        manifest.pop(k, None)


    try:
        for (shortpath, fullpath) in filepaths.items():
            zf.write(fullpath, arcname=shortpath, compress_type=compression)
        zf.writestr("manifest.json", json.dumps(dict(**manifest, version=version)))
    finally:
        zf.close()
    
    return _content_hash(dest)


def unpack_job(path, outdir):
    with zipfile.ZipFile(path, mode='r') as zf:
        zf.extractall(path=outdir)
        with open(Path(outdir) / 'manifest.json', 'r') as f:
            manifest = json.loads(f.read())
        return manifest, [n for n in zf.namelist() if Path(n).name != 'manifest.json']

    
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
