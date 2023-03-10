import os
import tempfile
from pathlib import Path
from .proc import ServerProcess, ServerProcessOpts
from enum import Enum

class P2PServerOpts(ServerProcessOpts):
    pass

class P2PServer():
    def __init__(self, opts, logger):
        self._logger = logger
        self._opts = opts
        self._proc = None

        # Using a temporary directory allows running multiple instances/queues
        # using the same filesystem (e.g. for development or containerized
        # farms)
        if None in (opts.driverCfg, opts.certsDir, opts.wwwCfg, opts.regDBWorld, opts.regDBLocal):
            self._tmpdir = tempfile.TemporaryDirectory()
            if self._opts.driverCfg is None:
                self._opts.driverCfg = f"{self._tmpdir.name}/driver.yaml"
            if self._opts.wwwCfg is None:
                self._opts.wwwCfg = f"{self._tmpdir.name}/www.yaml"
            if self._opts.regDBWorld is None:
                self._opts.wwwCfg = f"{self._tmpdir.name}/registry_world.sqlite3"
            if self._opts.regDBLocal is None:
                self._opts.wwwCfg = f"{self._tmpdir.name}/registry_local.sqlite3"
            if self._opts.certsDir is None:
                from .scripts.cert_gen import gen_certs
                self._opts.certsDir = f"{self._tmpdir.name}/certs/"
                os.mkdir(self._opts.certsDir)
                gen_certs(self._opts.certsDir, ncli=1)

        binpath = str((Path(__file__).parent / "server").absolute())
        self._logger.debug("initializing server process: " + binpath)
        self._proc = ServerProcess(self._opts, binpath, self._logger.getChild("proc"))
 
    def is_ready(self):
        return self._proc.is_running()

