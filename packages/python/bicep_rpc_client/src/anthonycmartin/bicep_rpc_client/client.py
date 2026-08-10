from __future__ import annotations

import json
import os
import platform
import stat
import subprocess
import threading
import urllib.request
from dataclasses import asdict
from pathlib import Path
from typing import Any, BinaryIO, Self, cast

from .models import (
    BicepClientConfiguration,
    CompileParamsRequest,
    CompileParamsResponse,
    CompileRequest,
    CompileResponse,
    FormatRequest,
    FormatResponse,
    GetDeploymentGraphRequest,
    GetDeploymentGraphResponse,
    GetFileReferencesRequest,
    GetFileReferencesResponse,
    GetMetadataRequest,
    GetMetadataResponse,
    GetSnapshotRequest,
    GetSnapshotResponse,
)


class RpcError(RuntimeError):
    def __init__(self, code: int, message: str, data: Any = None) -> None:
        super().__init__(f"Bicep RPC error {code}: {message}")
        self.code = code
        self.message = message
        self.data = data


class BicepClientFactory:
    def initialize(
        self, configuration: BicepClientConfiguration | None = None
    ) -> BicepClient:
        configuration = configuration or BicepClientConfiguration()
        executable = (
            Path(configuration.existing_cli_path).expanduser().resolve()
            if configuration.existing_cli_path is not None
            else _install_bicep(configuration.bicep_version, configuration.cache_root)
        )
        client = BicepClient(executable)
        try:
            client.get_version()
        except BaseException:
            client.close()
            raise
        return client


class BicepClient:
    def __init__(self, executable: str | Path) -> None:
        self._process = subprocess.Popen(
            [str(executable), "jsonrpc", "--stdio"],
            stdin=subprocess.PIPE,
            stdout=subprocess.PIPE,
            stderr=subprocess.DEVNULL,
        )
        self._input = cast(BinaryIO, self._process.stdin)
        self._output = cast(BinaryIO, self._process.stdout)
        if self._process.stdin is None or self._process.stdout is None:
            self.close()
            raise RuntimeError("Bicep JSON-RPC process did not expose stdin and stdout")
        self._lock = threading.Lock()
        self._next_id = 0
        self._version: str | None = None

    def compile(self, request: CompileRequest) -> CompileResponse:
        result = self._call("bicep/compile", {"path": str(Path(request.path).resolve())})
        return CompileResponse(
            success=result["success"],
            diagnostics=tuple(result.get("diagnostics", ())),
            contents=result.get("contents"),
        )

    def compile_params(self, request: CompileParamsRequest) -> CompileParamsResponse:
        result = self._call(
            "bicep/compileParams",
            {
                "path": str(Path(request.path).resolve()),
                "parameterOverrides": request.parameter_overrides,
            },
        )
        return CompileParamsResponse(
            success=result["success"],
            diagnostics=tuple(result.get("diagnostics", ())),
            parameters=result.get("parameters"),
            template=result.get("template"),
            template_spec_id=result.get("templateSpecId"),
        )

    def format(self, request: FormatRequest) -> FormatResponse:
        self._require_version("0.37.1", "format")
        result = self._call("bicep/format", {"path": str(Path(request.path).resolve())})
        return FormatResponse(contents=result["contents"])

    def get_metadata(self, request: GetMetadataRequest) -> GetMetadataResponse:
        result = self._call("bicep/getMetadata", {"path": str(Path(request.path).resolve())})
        return GetMetadataResponse(
            parameters=tuple(result.get("parameters", ())),
            outputs=tuple(result.get("outputs", ())),
            exports=tuple(result.get("exports", ())),
            metadata=tuple(result.get("metadata", ())),
        )

    def get_file_references(
        self, request: GetFileReferencesRequest
    ) -> GetFileReferencesResponse:
        result = self._call(
            "bicep/getFileReferences", {"path": str(Path(request.path).resolve())}
        )
        return GetFileReferencesResponse(file_paths=tuple(result.get("filePaths", ())))

    def get_deployment_graph(
        self, request: GetDeploymentGraphRequest
    ) -> GetDeploymentGraphResponse:
        result = self._call(
            "bicep/getDeploymentGraph", {"path": str(Path(request.path).resolve())}
        )
        return GetDeploymentGraphResponse(
            nodes=tuple(result.get("nodes", ())), edges=tuple(result.get("edges", ()))
        )

    def get_snapshot(self, request: GetSnapshotRequest) -> GetSnapshotResponse:
        self._require_version("0.36.1", "get_snapshot")
        metadata = {
            _snake_to_camel(name): value
            for name, value in asdict(request.metadata).items()
            if value is not None
        }
        external_inputs = [
            {
                _snake_to_camel(name): value
                for name, value in asdict(item).items()
                if value is not None
            }
            for item in request.external_inputs
        ]
        result = self._call(
            "bicep/getSnapshot",
            {
                "path": str(Path(request.path).resolve()),
                "metadata": metadata,
                "externalInputs": external_inputs,
            },
        )
        return GetSnapshotResponse(snapshot=result["snapshot"])

    def get_version(self) -> str:
        if self._version is None:
            self._version = self._call("bicep/version", {})["version"]
        return self._version

    def close(self) -> None:
        try:
            if self._process.poll() is None:
                self._process.terminate()
                try:
                    self._process.wait(timeout=5)
                except subprocess.TimeoutExpired:
                    self._process.kill()
                    self._process.wait()
        finally:
            input_stream = getattr(self, "_input", None)
            output_stream = getattr(self, "_output", None)
            if input_stream is not None:
                input_stream.close()
            if output_stream is not None:
                output_stream.close()

    def __enter__(self) -> Self:
        return self

    def __exit__(self, *_: object) -> None:
        self.close()

    def _require_version(self, minimum: str, operation: str) -> None:
        actual = self.get_version()
        if _version_tuple(actual) < _version_tuple(minimum):
            raise RuntimeError(
                f"Bicep CLI {minimum} or later is required for {operation}; detected {actual}"
            )

    def _call(self, method: str, params: dict[str, Any]) -> Any:
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
                    raise RpcError(error.get("code", 0), error.get("message", ""), error.get("data"))
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


def _install_bicep(version: str | None, cache_root: str | Path | None) -> Path:
    tag = f"v{version}" if version else _latest_version()
    root = Path(cache_root).expanduser() if cache_root is not None else Path.home() / ".bicep" / "bin"
    directory = root / tag
    directory.mkdir(parents=True, exist_ok=True)
    executable = directory / ("bicep.exe" if os.name == "nt" else "bicep")
    if executable.exists():
        return executable
    temporary = executable.with_suffix(executable.suffix + ".download")
    try:
        urllib.request.urlretrieve(
            f"https://downloads.bicep.azure.com/{tag}/{_artifact_name()}", temporary
        )
        temporary.chmod(temporary.stat().st_mode | stat.S_IXUSR | stat.S_IXGRP | stat.S_IXOTH)
        temporary.replace(executable)
    finally:
        temporary.unlink(missing_ok=True)
    return executable


def _latest_version() -> str:
    with urllib.request.urlopen("https://downloads.bicep.azure.com/releases/latest") as response:
        return json.load(response)["tag_name"]


def _artifact_name() -> str:
    operating_system = platform.system().lower()
    architecture = platform.machine().lower()
    architecture_name = {
        "amd64": "x64",
        "x86_64": "x64",
        "arm64": "arm64",
        "aarch64": "arm64",
    }.get(architecture)
    system_name = {"windows": "win", "linux": "linux", "darwin": "osx"}.get(operating_system)
    if system_name is None or architecture_name is None:
        raise RuntimeError(f"Bicep CLI is not available for {operating_system}/{architecture}")
    return f"bicep-{system_name}-{architecture_name}{'.exe' if operating_system == 'windows' else ''}"


def _version_tuple(version: str) -> tuple[int, ...]:
    return tuple(int(part.split("-")[0].split("+")[0]) for part in version.split("."))


def _snake_to_camel(value: str) -> str:
    first, *rest = value.split("_")
    return first + "".join(part.title() for part in rest)