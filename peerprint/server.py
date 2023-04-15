import os
import tempfile
from pathlib import Path
from .proc import ServerProcess, ServerProcessOpts
from enum import Enum
import platform

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

        binName = f"server-{self.get_system()}-{self.get_arch()}"
        if self.get_system() == 'windows':
            binName += ".exe"
        binpath = str((Path(__file__).parent / "bin" / binName).absolute())
        self._logger.debug("initializing server process: " + binpath)
        self._proc = ServerProcess(self._opts, binpath, self._logger.getChild("proc"))
 
    def get_system(self):
        s = platform.system().lower()
        VALID_SYS = ('windows', 'linux')
        if s not in VALID_SYS:
            raise Exception(f"platform.system() = {s}; must be one of {VALID_SYS}")
        return s

    def get_arch(self):
        a = platform.machine()
        if a == 'x86_64':
            a = 'amd64'
        elif a in ('aarch64_be', 'aarch64', 'armv8b', 'armv8l'):
            a = 'arm64'
        VALID_ARCHS = ('amd64', 'arm64')
        if a not in VALID_ARCHS:
            raise Exception(f"platform.machine() = {a}; must be one of {VALID_ARCHS}")
        return a

    def is_ready(self):
        return self._proc.is_running()

