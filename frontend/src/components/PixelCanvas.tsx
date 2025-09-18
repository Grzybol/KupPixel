import { MouseEvent, useEffect, useMemo, useRef } from "react";

export type Pixel = {
  id: number;
  status: "free" | "taken";
  color?: string;
  url?: string;
};

export type PixelCanvasProps = {
  width: number;
  height: number;
  pixels: Pixel[];
  onPixelClick: (pixel: Pixel) => void;
};

const FREE_COLOR: [number, number, number] = [55, 65, 81];

function normalizeHex(color?: string): string | undefined {
  if (!color) return undefined;
  if (color.startsWith("#")) {
    if (color.length === 4) {
      const r = color[1];
      const g = color[2];
      const b = color[3];
      return `#${r}${r}${g}${g}${b}${b}`;
    }
    return color.slice(0, 7);
  }
  return undefined;
}

function hexToRGB(color?: string): [number, number, number] {
  const normalized = normalizeHex(color);
  if (!normalized) {
    return FREE_COLOR;
  }
  const value = parseInt(normalized.slice(1), 16);
  const r = (value >> 16) & 0xff;
  const g = (value >> 8) & 0xff;
  const b = value & 0xff;
  return [r, g, b];
}

export default function PixelCanvas({ width, height, pixels, onPixelClick }: PixelCanvasProps) {
  const canvasRef = useRef<HTMLCanvasElement | null>(null);

  const data = useMemo(() => pixels, [pixels]);

  useEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas) return;
    const ctx = canvas.getContext("2d");
    if (!ctx) return;

    const imageData = ctx.createImageData(width, height);
    for (let i = 0; i < data.length; i++) {
      const pixel = data[i];
      const [r, g, b] = pixel.status === "taken" ? hexToRGB(pixel.color) : FREE_COLOR;
      const index = i * 4;
      imageData.data[index] = r;
      imageData.data[index + 1] = g;
      imageData.data[index + 2] = b;
      imageData.data[index + 3] = 255;
    }
    ctx.putImageData(imageData, 0, 0);
  }, [data, width, height]);

  const handleClick = (event: MouseEvent<HTMLCanvasElement>) => {
    const canvas = canvasRef.current;
    if (!canvas) return;
    const rect = canvas.getBoundingClientRect();
    const scaleX = canvas.width / rect.width;
    const scaleY = canvas.height / rect.height;
    const x = Math.floor((event.clientX - rect.left) * scaleX);
    const y = Math.floor((event.clientY - rect.top) * scaleY);
    const index = y * width + x;
    const pixel = data[index];
    if (pixel) {
      onPixelClick(pixel);
    }
  };

  return (
    <canvas
      ref={canvasRef}
      width={width}
      height={height}
      onClick={handleClick}
      className="w-full max-w-4xl border border-slate-700 rounded-lg shadow-md"
      style={{
        imageRendering: "pixelated",
        aspectRatio: `${width} / ${height}`,
        backgroundColor: "#111827"
      }}
    />
  );
}
