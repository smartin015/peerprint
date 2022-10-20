import subprocess
import atexit
import signal
from dataclasses import dataclass, fields

@dataclass
class ServerProcessOpts():
    # See ../server/main.go for flag defs and defaults
    addr: str = None
    raftAddr: str = None
    raftPath: str = None
    registry: str = None
    queue: str = None
    ipfs_server: str = None
    local: bool = None
    privkeyfile: str = None
    pubkeyfile: str = None
    connectTimeout: str = None
    zmq: str = None
    zmqpush: str = None
    zmqlog: str = None
    bootstrap: bool = None

    def render(self):
        args = ["peerprint_server"]
        for field in fields(self):
            val = getattr(self, field.name)
            if val is not None:
                args.append(f"-{field.name}={val}")
        return args

class ServerProcess():
    def __init__(self, opts, logger):
        self._logger = logger
        args = opts.render()
        self._proc = subprocess.Popen(args)
        self._logger.debug(f"Launch: {args}")
        atexit.register(self.destroy)

    def _signal(self, sig, timeout=5):
        self._proc.send_signal(signal.SIGINT)
        try:
            self._proc.wait(timeout)
        except subprocess.TimeoutExpired:
            pass
        return self._proc.returncode is not None

    def destroy(self):
        atexit.unregister(self.destroy)

        if self._proc is None or self._proc.returncode is not None:
            return
        self._logger.info(f"PID {self._proc.pid} (SIGINT)")
        if self._signal(signal.SIGINT):
            return

        self._logger.info(f"PID {self._proc.pid} (SIGKILL)")
        if self._signal(signal.SIGKILL):
            return

        self._logger.info(f"PID {self._proc.pid} (SIGTERM)")
        self._signal(signal.SIGKILL, timeout=None)

    def is_ready(self):
        return self._proc.returncode is None
