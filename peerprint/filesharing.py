import hashlib
import requests
import zipfile
import shutil
import os
import json
import time
import re
import tempfile
from .wan.ipfs import IPFS
from abc import ABC, abstractmethod
from pathlib import Path
from .version import __version__ as version
import http.server
import socketserver
import threading

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
def packed_name(s, basedir):
    if isinstance(basedir, str):
        basedir = Path(basedir)
    if s.strip() == "":
        s = "untitled"

    # Replace all non-word characters (everything except numbers and letters)
    s = re.sub(r"[^\w\s]", 'x', s)
    # Replace all runs of whitespace with underscore
    s = re.sub(r"\s+", '_', s)
    
    # Name suffix inspired by https://stackoverflow.com/a/57896232
    path = basedir / f"{s}.gjob" 
    counter = 1
    while path.exists():
        path = basedir / f"{s} ({counter}).gjob"
        counter += 1
    return str(path)


def pack_job(manifest: dict, filepaths: dict, dest: str):
    # TODO validation - ensure the correct files are available given the manifest object)
    zf = zipfile.ZipFile(dest, mode='w')

    # Sanitize short paths
    filepaths = dict([(short.split("/")[-1], full) for (short, full) in filepaths.items()])

    # Strip off paths in manifest (paths are stripped in zip file as well)
    # and remove unnecessary state/identity fields.
    for s in manifest["sets"]:
        s["path"] = s["path"].split("/")[-1]
        if filepaths.get(s["path"]) is None:
            raise ValueError(f"Job contains set with path={s['path']}, but filepaths has no matching short name")
        for k in ("sd"):
            # Note: we leave ID and rank around here as it's useful for ordering/referral to set items
            # Also leave "remaining" so that dragging between local and LAN queues is consistent.
            s.pop(k, None)
    for k in ("acquired", "id", "queue"):
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

class IPFSFileshare():
    def __init__(self, basedir, logger):
        self.basedir = basedir
        self._logger = logger
        os.makedirs(basedir, exist_ok=True)
        self.proc = IPFS.start_daemon()

    def post(self, manifest: dict, filepaths: dict) -> str:
        with tempfile.NamedTemporaryFile(suffix='.gjob', dir=self.basedir, delete=False) as tf:
            hash_ = pack_job(manifest, filepaths, tf.name)
            ipfs_cid = IPFS.add(tf.name).decode('utf8')
            dest = Path(self.basedir) / f"{ipfs_cid}.gjob"
            os.rename(tf.name, dest)
            self._logger.info(f"Packed and posted job to {dest} - IPFS id {ipfs_cid}")
            return ipfs_cid

    def fetch(self, hash_:str, unpack=False, overwrite=False) -> str:
        # Get the equivalent path on disk
        name = f"{hash_}.gjob"
        dest = Path(self.basedir) / name
        if dest.exists() and not overwrite:
            self._logger.debug("File already exists - using that one")
        else:
            if not IPFS.fetch(hash_, dest):
                raise Exception("Failed to fetch file {hash_}")

        if unpack:
            dest_dir = Path(self.basedir) / hash_
            if not dest_dir.exists() or overwrite:
                unpack_job(dest, dest_dir)
            return dest_dir
        else:
            return dest


class FileshareServer(socketserver.TCPServer):
    allow_reuse_address = True

class Fileshare:
    def __init__(self, addr, basedir, logger):
        (host, port) = addr.split(":")
        self.host = host
        self.port = int(port)
        self.basedir = basedir
        self.t = None
        self._logger = logger
        os.makedirs(basedir, exist_ok=True)

    def connect(self, testing=False):
        if testing:
            return
        basedir = self.basedir
        class FileshareRequestHandler(http.server.SimpleHTTPRequestHandler):
            def __init__(self, *args, **kwargs):
                super().__init__(*args, directory=basedir, **kwargs)

        self.httpd = FileshareServer((self.host, self.port), FileshareRequestHandler)
        self.httpd.allow_reuse_address = True
        self.t = threading.Thread(target=self.httpd.serve_forever, daemon=True)
        self.t.start()

        # Re-assign host & port since some formats (e.g. *:0) auto-assign port
        (self.host, self.port) = self.httpd.socket.getsockname()

    def destroy(self):
        if self.httpd is not None:
            self._logger.info("Fileshare shutting down")
            self.httpd.shutdown()
            self.httpd.server_close()
            self._logger.info("Server shut down")

    def post(self, manifest: dict, filepaths: dict) -> str:
        # We must first write to a temp file so we can calculate the hash and use it
        # as the destination file name
        # Note that basedir must be used when creating the file, as it otherwise defaults to /tmp
        # which is in-memory and causes os.rename to fail due to cross-device linking.
        with tempfile.NamedTemporaryFile(suffix='.gjob', dir=self.basedir, delete=False) as tf:
            hash_ = pack_job(manifest, filepaths, tf.name)
            dest = Path(self.basedir) / f"{hash_}.gjob"
            os.rename(tf.name, dest)
            self._logger.info(f"Packed and posted job to {dest}")
            return hash_

    def fetch(self, peer:str, hash_:str, unpack=False, overwrite=False) -> str:
        # Get the equivalent path on disk
        written = 0
        name = f"{hash_}.gjob"
        url = f"http://{peer}/{name}"
        dest = Path(self.basedir) / name
        self._logger.debug(f"HTTP GET {url} -> {dest}")

        if dest.exists() and not overwrite:
            self._logger.debug("File already exists - using that one")
        else:
            with requests.get(url, stream=True) as r:
              r.raise_for_status()
              with open(dest, 'wb') as f:
                  for chunk in r.iter_content(chunk_size=8192):
                      f.write(chunk)
                      written += len(chunk)
            self._logger.debug(f"Wrote {written}B to {dest}")

        if unpack:
            dest_dir = Path(self.basedir) / hash_
            if not dest_dir.exists() or overwrite:
                unpack_job(dest, dest_dir)
            return dest_dir
        else:
            return dest
