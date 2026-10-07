import struct

import numpy as np
import pytest

from spill_glass_finder.pcd import decode_pcd, encode_pcd


def _header(fields: str, sizes: str, types: str, n: int, mode: str) -> bytes:
    counts = " ".join("1" for _ in fields.split())
    return (
        f"# .PCD v0.7\nVERSION .7\nFIELDS {fields}\nSIZE {sizes}\nTYPE {types}\nCOUNT {counts}\n"
        f"WIDTH {n}\nHEIGHT 1\nVIEWPOINT 0 0 0 1 0 0 0\nPOINTS {n}\nDATA {mode}\n"
    ).encode()


POINTS_MM = np.array([[1.0, -2.5, 300.0], [-150.25, 75.5, 1200.0], [0.0, 0.0, 1.0]])


def test_binary_round_trip() -> None:
    np.testing.assert_allclose(decode_pcd(encode_pcd(POINTS_MM)), POINTS_MM, atol=1e-3)


def test_encode_writes_meters() -> None:
    data = encode_pcd(POINTS_MM[:1])
    payload = data[data.index(b"DATA binary\n") + len(b"DATA binary\n") :]
    np.testing.assert_allclose(struct.unpack("<3f", payload), POINTS_MM[0] / 1000.0, rtol=1e-6)


def test_empty_round_trip() -> None:
    assert decode_pcd(encode_pcd(np.zeros((0, 3)))).shape == (0, 3)


def test_binary_with_rgb() -> None:
    meters = POINTS_MM / 1000.0
    payload = b"".join(struct.pack("<fffI", *p, 0x00FF8800) for p in meters)
    data = _header("x y z rgb", "4 4 4 4", "F F F I", len(meters), "binary") + payload
    np.testing.assert_allclose(decode_pcd(data), POINTS_MM, atol=1e-3)


def test_binary_with_leading_field_and_float64() -> None:
    meters = POINTS_MM / 1000.0
    payload = b"".join(struct.pack("<Hddd", 7, *p) for p in meters)
    data = _header("intensity x y z", "2 8 8 8", "U F F F", len(meters), "binary") + payload
    np.testing.assert_allclose(decode_pcd(data), POINTS_MM, atol=1e-9)


@pytest.mark.parametrize("with_rgb", [False, True])
def test_ascii(with_rgb: bool) -> None:
    meters = POINTS_MM / 1000.0
    if with_rgb:
        rows = "".join(f"{x} {y} {z} 16711680\n" for x, y, z in meters)
        data = _header("x y z rgb", "4 4 4 4", "F F F I", len(meters), "ascii") + rows.encode()
    else:
        rows = "".join(f"{x} {y} {z}\n" for x, y, z in meters)
        data = _header("x y z", "4 4 4", "F F F", len(meters), "ascii") + rows.encode()
    np.testing.assert_allclose(decode_pcd(data), POINTS_MM, atol=1e-9)


def test_rejects_compressed() -> None:
    with pytest.raises(ValueError, match="binary_compressed"):
        decode_pcd(_header("x y z", "4 4 4", "F F F", 1, "binary_compressed") + b"\x00" * 12)


def test_rejects_truncated_binary() -> None:
    with pytest.raises(ValueError, match="expected"):
        decode_pcd(_header("x y z", "4 4 4", "F F F", 2, "binary") + b"\x00" * 12)
