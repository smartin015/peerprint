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
        if opts.baseDir is None:
            self._tmpdir = tempfile.TemporaryDirectory()
            opts.baseDir = self._tmpdir.name

        binpath = str((Path(__file__).parent / "server").absolute())
        self._logger.debug("initializing server process: " + binpath)
        self._proc = ServerProcess(self._opts, binpath, self._logger.getChild("proc"))
 
    def is_ready(self):
        return self._proc.is_running()

