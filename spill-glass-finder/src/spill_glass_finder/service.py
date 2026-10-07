import asyncio
import threading
from dataclasses import dataclass
from functools import lru_cache
from typing import Any, ClassVar, List, Mapping, Optional, Self, Sequence, Tuple

import numpy as np
from numpy.typing import NDArray
from viam.components.camera import Camera
from viam.logging import getLogger
from viam.media.utils.pil import viam_to_pil_image
from viam.media.video import CameraMimeType, ViamImage
from viam.proto.app.robot import ComponentConfig
from viam.proto.common import GeometriesInFrame, Geometry, PointCloudObject, Pose, ResourceName, Sphere
from viam.proto.service.vision import Classification, Detection, Detection3D
from viam.resource.base import ResourceBase
from viam.resource.easy_resource import EasyResource
from viam.resource.types import Model, ModelFamily
from viam.services.vision import CaptureAllResult, Vision
from viam.utils import ValueTypes, struct_to_dict

from .config import GlassFinderConfig, parse_config
from .detections import FrameDetections
from .pcd import decode_pcd, encode_pcd
from .pipeline import FindResult, find_glasses, points_in_box, sample_cylinder, to_world_mm
from .spill.glassloc import GlassLocalizer
from .types import KEYPOINT_NAMES, BoxDetection, GlassEstimate, Intrinsics

LOGGER = getLogger(__name__)

KEYPOINT_BOX_HALF_SIZE_PX = 4
RIM_MARKER_RADIUS_MM = 5.0
WORLD = "world"


@dataclass
class _Spill:
    localizer: GlassLocalizer
    frame_detections: FrameDetections
    lock: threading.Lock


@lru_cache(maxsize=1)
def _load_spill(checkpoint_path: str, labels: tuple[str, ...]) -> _Spill:
    """Cached so the frequent resource rebuilds don't reload the checkpoint."""
    frame_detections = FrameDetections()
    localizer = GlassLocalizer(np.eye(3), checkpoint_path, list(labels), frame_detections)
    return _Spill(localizer=localizer, frame_detections=frame_detections, lock=threading.Lock())


def glass_label(glass: GlassEstimate) -> str:
    return f"{glass.label} r={glass.radius_mm:.0f}mm h={glass.height_mm:.0f}mm"


def glass_to_dict(glass: GlassEstimate) -> dict[str, Any]:
    return {
        "label": glass.label,
        "score": glass.score,
        "bbox": list(glass.bbox),
        "keypoints_px": glass.keypoints_dict(),
        "rim_center_mm": [float(v) for v in glass.rim_center_mm],
        "radius_mm": glass.radius_mm,
        "height_mm": glass.height_mm,
        "tilt_deg": glass.tilt_deg,
        "frame": WORLD,
    }


def result_to_dict(result: FindResult) -> dict[str, Any]:
    return {
        "glasses": [glass_to_dict(g) for g in result.glasses],
        "table": {"height_mm": result.table_height_mm, "frame": WORLD},
    }


def glass_detections(glass: GlassEstimate) -> list[Detection]:
    x0, y0, x1, y1 = glass.bbox
    detections = [
        Detection(x_min=x0, y_min=y0, x_max=x1, y_max=y1, confidence=glass.score, class_name=glass_label(glass))
    ]
    for name, uv in zip(KEYPOINT_NAMES, glass.keypoints_px):
        u, v = (int(round(c)) for c in uv)
        detections.append(
            Detection(
                x_min=max(u - KEYPOINT_BOX_HALF_SIZE_PX, 0),
                y_min=max(v - KEYPOINT_BOX_HALF_SIZE_PX, 0),
                x_max=u + KEYPOINT_BOX_HALF_SIZE_PX,
                y_max=v + KEYPOINT_BOX_HALF_SIZE_PX,
                confidence=glass.score,
                class_name=f"kp-{name}",
            )
        )
    return detections


def _object(points_mm: NDArray[np.float64], center_mm: NDArray[np.float64], label: str, frame: str) -> PointCloudObject:
    x, y, z = (float(c) for c in center_mm)
    return PointCloudObject(
        point_cloud=encode_pcd(points_mm),
        geometries=GeometriesInFrame(
            reference_frame=frame,
            geometries=[
                Geometry(
                    center=Pose(x=x, y=y, z=z, o_x=0.0, o_y=0.0, o_z=1.0, theta=0.0),
                    sphere=Sphere(radius_mm=RIM_MARKER_RADIUS_MM),
                    label=label,
                )
            ],
        ),
    )


class SpillGlassFinder(Vision, EasyResource):
    MODEL: ClassVar[Model] = Model(ModelFamily("viam", "cocktail-bot"), "spill-glass-finder")

    config: GlassFinderConfig
    camera: Camera
    detector: Vision
    spill: _Spill

    @classmethod
    def new(cls, config: ComponentConfig, dependencies: Mapping[ResourceName, ResourceBase]) -> Self:
        self = cls(config.name)
        self.config = parse_config(struct_to_dict(config.attributes))
        self.camera = dependencies[Camera.get_resource_name(self.config.camera_name)]  # type: ignore[assignment]
        self.detector = dependencies[Vision.get_resource_name(self.config.detector_name)]  # type: ignore[assignment]
        self.spill = _load_spill(self.config.checkpoint_path, self.config.labels)
        return self

    @classmethod
    def validate_config(cls, config: ComponentConfig) -> Tuple[Sequence[str], Sequence[str]]:
        parsed = parse_config(struct_to_dict(config.attributes))
        return [parsed.camera_name, parsed.detector_name], []

    def _check_camera(self, camera_name: str) -> None:
        if camera_name not in ("", self.config.camera_name):
            raise ValueError(f"this service is configured for camera {self.config.camera_name!r}, got {camera_name!r}")

    async def _capture_image(self) -> ViamImage:
        images, _ = await self.camera.get_images()
        for image in images:
            if image.mime_type in (CameraMimeType.JPEG, CameraMimeType.PNG, CameraMimeType.VIAM_RGBA):
                return image
        raise ValueError(
            f"camera {self.config.camera_name!r} returned no color image ({[i.mime_type for i in images]})"
        )

    async def _intrinsics(self) -> Intrinsics:
        props = await self.camera.get_properties()
        p = props.intrinsic_parameters
        if p.width_px <= 0 or p.height_px <= 0 or p.focal_x_px <= 0 or p.focal_y_px <= 0:
            raise ValueError(f"camera {self.config.camera_name!r} reports no intrinsics")
        return Intrinsics(p.width_px, p.height_px, p.focal_x_px, p.focal_y_px, p.center_x_px, p.center_y_px)

    async def _point_cloud(self) -> NDArray[np.float64]:
        data, mime_type = await self.camera.get_point_cloud()
        if mime_type != CameraMimeType.PCD:
            raise ValueError(f"camera {self.config.camera_name!r} returned point cloud mime type {mime_type!r}")
        return decode_pcd(data)

    def _locked_find(
        self,
        image_bgr: NDArray[np.uint8],
        cloud_mm: NDArray[np.float64],
        intrinsics: Intrinsics,
        detections: list[BoxDetection],
    ) -> FindResult:
        with self.spill.lock:
            return find_glasses(
                self.spill.localizer,
                self.spill.frame_detections,
                image_bgr,
                cloud_mm,
                intrinsics,
                detections,
                self.config,
            )

    async def _run(self, image: ViamImage) -> tuple[FindResult, NDArray[np.float64], Intrinsics]:
        cloud_mm, intrinsics, raw_detections = await asyncio.gather(
            self._point_cloud(), self._intrinsics(), self.detector.get_detections(image)
        )
        # SPILL's localize_glass takes an OpenCV (BGR) image.
        image_bgr = np.ascontiguousarray(np.asarray(viam_to_pil_image(image).convert("RGB"))[:, :, ::-1])
        detections = [
            BoxDetection(d.class_name, d.confidence, d.x_min, d.y_min, d.x_max, d.y_max) for d in raw_detections
        ]
        result = await asyncio.to_thread(self._locked_find, image_bgr, cloud_mm, intrinsics, detections)
        for i, glass in enumerate(result.glasses):
            LOGGER.debug("glass %d: %s", i, glass_to_dict(glass))
        return result, cloud_mm, intrinsics

    def _objects(
        self, result: FindResult, cloud_mm: NDArray[np.float64], intrinsics: Intrinsics, debug: bool
    ) -> list[PointCloudObject]:
        objects = [
            _object(
                sample_cylinder(g.rim_center_mm, g.radius_mm, g.height_mm, self.config.surface_points),
                g.rim_center_mm,
                glass_label(g),
                WORLD,
            )
            for g in result.glasses
        ]
        if debug:
            objects += [
                _object(
                    to_world_mm(points_in_box(cloud_mm, intrinsics, g.bbox), self.config.world_from_camera_m),
                    g.rim_center_mm,
                    f"{glass_label(g)}-raw",
                    WORLD,
                )
                for g in result.glasses
            ]
        return objects

    async def find(self) -> FindResult:
        result, _, _ = await self._run(await self._capture_image())
        return result

    async def capture_all_from_camera(
        self,
        camera_name: str,
        return_image: bool = False,
        return_classifications: bool = False,
        return_detections: bool = False,
        return_object_point_clouds: bool = False,
        return_detections_3d: bool = False,
        *,
        extra: Optional[Mapping[str, ValueTypes]] = None,
        timeout: Optional[float] = None,
    ) -> CaptureAllResult:
        self._check_camera(camera_name)
        if return_classifications or return_detections_3d:
            raise NotImplementedError("spill-glass-finder supports only detections and object point clouds")
        image = await self._capture_image()
        result, cloud_mm, intrinsics = await self._run(image)
        return CaptureAllResult(
            image=image if return_image else None,
            detections=[d for g in result.glasses for d in glass_detections(g)] if return_detections else None,
            objects=self._objects(result, cloud_mm, intrinsics, bool((extra or {}).get("debug")))
            if return_object_point_clouds
            else None,
        )

    async def get_detections_from_camera(
        self,
        camera_name: str,
        *,
        extra: Optional[Mapping[str, ValueTypes]] = None,
        timeout: Optional[float] = None,
    ) -> List[Detection]:
        self._check_camera(camera_name)
        result, _, _ = await self._run(await self._capture_image())
        return [d for g in result.glasses for d in glass_detections(g)]

    async def get_detections(
        self,
        image: ViamImage,
        *,
        extra: Optional[Mapping[str, ValueTypes]] = None,
        timeout: Optional[float] = None,
    ) -> List[Detection]:
        """Uses the given image with the configured camera's current point cloud and intrinsics."""
        result, _, _ = await self._run(image)
        return [d for g in result.glasses for d in glass_detections(g)]

    async def get_classifications_from_camera(
        self,
        camera_name: str,
        count: int,
        *,
        extra: Optional[Mapping[str, ValueTypes]] = None,
        timeout: Optional[float] = None,
    ) -> List[Classification]:
        raise NotImplementedError("spill-glass-finder does not support classifications")

    async def get_classifications(
        self,
        image: ViamImage,
        count: int,
        *,
        extra: Optional[Mapping[str, ValueTypes]] = None,
        timeout: Optional[float] = None,
    ) -> List[Classification]:
        raise NotImplementedError("spill-glass-finder does not support classifications")

    async def get_detections_3d(
        self,
        camera_name: str,
        *,
        extra: Optional[Mapping[str, ValueTypes]] = None,
        timeout: Optional[float] = None,
    ) -> List[Detection3D]:
        raise NotImplementedError("spill-glass-finder does not support 3D detections; use get_object_point_clouds")

    async def get_object_point_clouds(
        self,
        camera_name: str,
        *,
        extra: Optional[Mapping[str, ValueTypes]] = None,
        timeout: Optional[float] = None,
    ) -> List[PointCloudObject]:
        self._check_camera(camera_name)
        result, cloud_mm, intrinsics = await self._run(await self._capture_image())
        return self._objects(result, cloud_mm, intrinsics, bool((extra or {}).get("debug")))

    async def get_properties(
        self,
        *,
        extra: Optional[Mapping[str, ValueTypes]] = None,
        timeout: Optional[float] = None,
    ) -> Vision.Properties:
        return Vision.Properties(
            classifications_supported=False,
            detections_supported=True,
            object_point_clouds_supported=True,
            detections_3d_supported=False,
            default_camera=self.config.camera_name,
        )

    async def do_command(
        self,
        command: Mapping[str, ValueTypes],
        *,
        timeout: Optional[float] = None,
        **kwargs: Any,
    ) -> Mapping[str, ValueTypes]:
        if "find_glasses" in command:
            return result_to_dict(await self.find())
        raise ValueError(f"unknown command {sorted(command)}; supported: find_glasses")
