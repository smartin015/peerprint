import subprocess
import signal
import asyncio
from dataclasses import dataclass, fields

class ProcessOptsBase():
    def render(self, binary_path):
        args = [binary_path]
        for field in fields(self):
            val = getattr(self, field.name)
            if val is not None:
                args.append(f"-{field.name}={val}")
        return args

@dataclass
class ServerProcessOpts(ProcessOptsBase):
    # See cmd/server/main.go for flag defs and defaults
    baseDir: str = None
    addr: str = None
    certsDir: str = None
    connDir: str = None
    driverCfg: str = None
    rootCert: str = None
    regDBLocal: str = None
    regDBWorld: str = None
    serverCert: str = None
    serverKey: str = None
    www: str = None
    wwwCfg: str = None
    wwwDir: str = None


class IPFSDaemonProcess():
    def __init__(self, logger):
        self._logger = logger
        if self.is_running():
            self._logger.warning("IPFS daemon already running; skipping daemon creation")
            self._proc = None
        else:
            self._proc = subprocess.Popen(("ipfs", "daemon", "--init", "--enable-pubsub-experiment"))
            self._logger.info("Launched IPFS daemon")

    def is_running(self):
        p = subprocess.Popen(("ipfs", "stats", "bw"), stdout=subprocess.PIPE, stderr=subprocess.STDOUT)
        p.communicate()
        return (p.returncode == 0)


class ServerProcess():
    def __init__(self, opts, binary_path, logger):
        self._logger = logger
        args = opts.render(binary_path)
        self._proc = subprocess.Popen(args)
        self._logger.debug(f"Launch: {' '.join(args)}")

    def _signal(self, sig, timeout=1):
        if getattr(self, "_proc", None) is None or self._proc.returncode is not None:
            return True
        self._logger.info(f"SIGNAL PID {self._proc.pid} ({sig})")
        self._proc.send_signal(sig)
        try:
            self._proc.wait(timeout)
        except subprocess.TimeoutExpired:
            pass
        rc = self._proc.returncode
        return rc is not None

    def destroy(self):
        return self._signal(signal.SIGINT) or self._signal(signal.SIGKILL) or self._signal(signal.SIGTERM) 

    def is_running(self):
        return self._proc.returncode is None
