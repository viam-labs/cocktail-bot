import type { GlassImage } from "./lib/glasses";

const PATHS: Record<GlassImage, string> = {
  martini: "M5 8 H35 L20 30 Z M20 30 V53 M12 53 H28",
  coupe: "M5 14 Q5 31 20 31 Q35 31 35 14 Z M20 31 V53 M12 53 H28",
  highball: "M12 4 H28 L27 55 H13 Z",
  rocks: "M7 27 H33 L31 55 H9 Z",
  wine: "M12 12 H28 C33 22 35 34 33 44 C31 52 27 55 20 55 C13 55 9 52 7 44 C5 34 7 22 12 12 Z",
};

export function GlassIcon({ image, size = 32 }: { image?: GlassImage; size?: number }) {
  return (
    <svg
      viewBox="0 0 40 60"
      width={(size * 2) / 3}
      height={size}
      aria-hidden="true"
      style={{ flex: "none" }}
    >
      <path
        d={PATHS[image ?? "rocks"] ?? PATHS.rocks}
        fill="none"
        stroke="currentColor"
        strokeWidth={2.5}
        strokeLinejoin="round"
        strokeLinecap="round"
      />
    </svg>
  );
}
