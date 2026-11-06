"""Import smoke test for the generated Python SDK under numinance_sdk/gen.

Every .proto under numinance-spec/proto must have an importable <name>_pb2 and
<name>_pb2_grpc module, and every generated module must have a .proto. Run by
just gen-check with the locked SDK environment; standard library unittest only,
because pytest is not in uv.lock.
"""

import importlib
import pathlib
import sys
import unittest

_SDK = pathlib.Path(__file__).resolve().parents[1]
_GEN = _SDK / "numinance_sdk" / "gen"
_PROTO = _SDK.parents[1] / "proto"


def setUpModule() -> None:
    # The generated modules import each other as top-level numinance.*.
    sys.path.insert(0, str(_GEN))


def tearDownModule() -> None:
    sys.path.remove(str(_GEN))


def _module_name(rel: pathlib.PurePath) -> str:
    return ".".join(rel.with_suffix("").parts)


def _expected_modules() -> set[str]:
    names = set()
    for proto in _PROTO.rglob("*.proto"):
        stem = _module_name(proto.relative_to(_PROTO))
        names.add(stem + "_pb2")
        names.add(stem + "_pb2_grpc")
    return names


def _modules_on_disk() -> set[str]:
    return {_module_name(p.relative_to(_GEN)) for p in _GEN.rglob("*.py")}


class ImportSmokeTest(unittest.TestCase):
    def test_every_generated_module_imports(self) -> None:
        expected = _expected_modules()
        on_disk = _modules_on_disk()
        self.assertTrue(expected, f"no .proto files found under {_PROTO}")
        self.assertEqual(
            sorted(expected - on_disk), [], "modules missing: run just gen"
        )
        self.assertEqual(
            sorted(on_disk - expected), [], "modules with no .proto: delete them"
        )
        for name in sorted(expected):
            with self.subTest(module=name):
                importlib.import_module(name)

    def test_plugin_message_and_service_resolve(self) -> None:
        pb2 = importlib.import_module("numinance.plugin.v1.plugin_pb2")
        grpc_pb2 = importlib.import_module("numinance.plugin.v1.plugin_pb2_grpc")
        req = pb2.HandshakeRequest()
        self.assertEqual(
            req.DESCRIPTOR.full_name, "numinance.plugin.v1.HandshakeRequest"
        )
        service = pb2.DESCRIPTOR.services_by_name["PluginService"]
        self.assertEqual(service.full_name, "numinance.plugin.v1.PluginService")
        for symbol in (
            "PluginServiceStub",
            "PluginServiceServicer",
            "add_PluginServiceServicer_to_server",
        ):
            with self.subTest(symbol=symbol):
                self.assertTrue(
                    callable(getattr(grpc_pb2, symbol, None)),
                    f"plugin_pb2_grpc.{symbol} does not resolve",
                )


if __name__ == "__main__":
    unittest.main()
