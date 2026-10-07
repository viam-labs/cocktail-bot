import numpy as np
import pytest

from spill_glass_finder.pose import orientation_vector_degrees_to_matrix, pose_config_to_matrix_m

# Generated offline from go.viam.com/rdk v1.10.0 spatialmath.OrientationVectorDegrees.RotationMatrix().
RDK_REFERENCE = [
    ((0, 0, 1, 0), [[1, 0, 0], [0, 1, 0], [0, 0, 1]]),
    ((0, 0, 1, 90), [[0, -1, 0], [1, 0, 0], [0, 0, 1]]),
    ((1, 0, 0, 0), [[0, 0, 1], [0, 1, 0], [-1, 0, 0]]),
    ((0, 1, 0, 30), [[-0.5, -0.866025403784, 0], [0, 0, 1], [-0.866025403784, 0.5, 0]]),
    (
        (0.3, -0.5, -0.8, 120),
        [
            [0.950498335835, -0.068674441249, 0.303045763366],
            [0.099086930002, -0.857367913726, -0.505076272276],
            [0.294507544687, 0.510102030610, -0.808122035642],
        ],
    ),
    ((0, 0, -1, 45), [[-0.707106781187, 0.707106781187, 0], [0.707106781187, 0.707106781187, 0], [0, 0, -1]]),
    (
        (-0.2, 0.9, 0.1, -60),
        [
            [0.833706686536, -0.508351780678, -0.215665546407],
            [0.240499766258, -0.017303101270, 0.970494958831],
            [-0.497084523251, -0.860975649927, 0.107832773203],
        ],
    ),
]


@pytest.mark.parametrize("ov, expected", RDK_REFERENCE)
def test_matches_rdk(ov: tuple[float, float, float, float], expected: list[list[float]]) -> None:
    np.testing.assert_allclose(orientation_vector_degrees_to_matrix(*ov), expected, atol=1e-9)


def test_pose_config_to_matrix() -> None:
    transform = pose_config_to_matrix_m(
        {
            "translation": {"x": 100, "y": -20, "z": 1250},
            "orientation": {"type": "ov_degrees", "value": {"x": 1, "y": 0, "z": 0, "th": 0}},
        }
    )
    np.testing.assert_allclose(transform[:3, 3], [0.1, -0.02, 1.25])
    np.testing.assert_allclose(transform[:3, :3], RDK_REFERENCE[2][1], atol=1e-12)
    np.testing.assert_allclose(transform[3], [0, 0, 0, 1])


def test_pose_config_rejects_other_orientation_types() -> None:
    with pytest.raises(ValueError, match="ov_degrees"):
        pose_config_to_matrix_m({"translation": {}, "orientation": {"type": "euler_angles", "value": {}}})
