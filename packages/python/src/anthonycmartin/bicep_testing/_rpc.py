from __future__ import annotations

import json
import subprocess
import threading
from pathlib import Path
from typing import Any, BinaryIO


class RpcError(RuntimeError):
    pass


class RpcClient:
    def __init__(self, executable: Path) -> None:
        self._process = subprocess.Popen(
            [str(executable), "jsonrpc", "--stdio"],
            stdin=subprocess.PIPE,
            stdout=subprocess.PIPE,
            stderr=subprocess.DEVNULL,
        )
        if self._process.stdin is None or self._process.stdout is None:
            self.close()
            raise RuntimeError("Bicep JSON-RPC process did not expose stdin and stdout")
        self._input: BinaryIO = self._process.stdin
        self._output: BinaryIO = self._process.stdout
        self._lock = threading.Lock()
        self._next_id = 0

    def call(self, method: str, params: dict[str, Any]) -> Any:
        with self._lock:
            self._next_id += 1
            request_id = self._next_id
            body = json.dumps(
                {"jsonrpc": "2.0", "id": request_id, "method": method, "params": params},
                separators=(",", ":"),
            ).encode()
            self._input.write(f"Content-Length: {len(body)}\r\n\r\n".encode())
            self._input.write(body)
            self._input.flush()

            while True:
                response = self._read_message()
                if response.get("id") != request_id:
                    continue
                if error := response.get("error"):
                    raise RpcError(
                        f"Bicep RPC error {error.get('code')}: {error.get('message')}"
                    )
                return response.get("result")

    def _read_message(self) -> dict[str, Any]:
        content_length: int | None = None
        while True:
            line = self._output.readline()
            if not line:
                raise EOFError("Bicep JSON-RPC process closed its output")
            if line in (b"\r\n", b"\n"):
                break
            name, separator, value = line.decode().partition(":")
            if separator and name.strip().lower() == "content-length":
                content_length = int(value.strip())
        if content_length is None:
            raise RuntimeError("Bicep RPC response did not include Content-Length")
        return json.loads(self._output.read(content_length))

    def close(self) -> None:
        if self._process.poll() is not None:
            return
        self._process.terminate()
        try:
            self._process.wait(timeout=5)
        except subprocess.TimeoutExpired:
            self._process.kill()
            self._process.wait()