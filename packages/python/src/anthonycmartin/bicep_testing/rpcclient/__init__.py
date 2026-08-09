"""JSON-RPC transport for communicating with the Bicep CLI."""

from .client import RpcClient, RpcError

__all__ = ["RpcClient", "RpcError"]