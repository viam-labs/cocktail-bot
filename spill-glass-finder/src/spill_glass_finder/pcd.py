"""PCD codec for Viam point clouds.

PCD bytes on the wire are in meters (RDK's pointcloud.ToPCD divides by 1000 and ReadPCD multiplies back);
everything returned or accepted here is in millimeters.
"""

import numpy as np
from numpy.typing import NDArray

_NUMPY_TYPES: dict[tuple[str, int], str] = {
    ("F", 4): "f4",
    ("F", 8): "f8",
    ("I", 1): "i1",
    ("I", 2): "i2",
    ("I", 4): "i4",
    ("I", 8): "i8",
    ("U", 1): "u1",
    ("U", 2): "u2",
    ("U", 4): "u4",
    ("U", 8): "u8",
}


def decode_pcd(data: bytes) -> NDArray[np.float64]:
    """Return the (N, 3) xyz points in mm; extra fields are ignored."""
    header: dict[str, list[str]] = {}
    offset = 0
    while True:
        end = data.find(b"\n", offset)
        if end < 0:
            raise ValueError("PCD header has no DATA line")
        line = data[offset:end].decode("ascii").strip()
        offset = end + 1
        if not line or line.startswith("#"):
            continue
        key, *values = line.split()
        header[key.upper()] = values
        if key.upper() == "DATA":
            break

    fields = header["FIELDS"]
    sizes = [int(s) for s in header["SIZE"]]
    types = [t.upper() for t in header["TYPE"]]
    counts = [int(c) for c in header.get("COUNT", ["1"] * len(fields))]
    n_points = int(header["POINTS"][0])
    for axis in ("x", "y", "z"):
        if axis not in fields:
            raise ValueError(f"PCD has no {axis!r} field (fields: {fields})")

    mode = header["DATA"][0].lower()
    if mode == "binary":
        dtype = np.dtype(
            [
                (name, "<" + _NUMPY_TYPES[(t, s)], (c,)) if c > 1 else (name, "<" + _NUMPY_TYPES[(t, s)])
                for name, s, t, c in zip(fields, sizes, types, counts)
            ]
        )
        expected = n_points * dtype.itemsize
        if len(data) - offset < expected:
            raise ValueError(f"PCD binary payload is {len(data) - offset} bytes, expected {expected}")
        records = np.frombuffer(data, dtype=dtype, count=n_points, offset=offset)
        xyz = np.stack([records["x"], records["y"], records["z"]], axis=1).astype(np.float64)
    elif mode == "ascii":
        columns = np.cumsum([0] + counts)
        rows = data[offset:].decode("ascii").split()
        n_columns = int(columns[-1])
        if len(rows) < n_points * n_columns:
            raise ValueError(f"PCD ascii payload has {len(rows)} values, expected {n_points * n_columns}")
        table = np.asarray(rows[: n_points * n_columns], dtype=np.float64).reshape(n_points, n_columns)
        xyz = table[:, [int(columns[fields.index(axis)]) for axis in ("x", "y", "z")]]
    else:
        raise ValueError(f"unsupported PCD DATA mode {mode!r}; only ascii and binary are supported")
    return xyz * 1000.0


def encode_pcd(points_mm: NDArray[np.float64]) -> bytes:
    """Encode (N, 3) mm points as a binary x y z float32 PCD in meters."""
    points_m = (np.asarray(points_mm, dtype=np.float64).reshape(-1, 3) / 1000.0).astype("<f4")
    n = points_m.shape[0]
    header = (
        "VERSION .7\n"
        "FIELDS x y z\n"
        "SIZE 4 4 4\n"
        "TYPE F F F\n"
        "COUNT 1 1 1\n"
        f"WIDTH {n}\n"
        "HEIGHT 1\n"
        "VIEWPOINT 0 0 0 1 0 0 0\n"
        f"POINTS {n}\n"
        "DATA binary\n"
    )
    return header.encode("ascii") + points_m.tobytes()
