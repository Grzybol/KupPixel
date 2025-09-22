import { MouseEvent, useEffect, useMemo, useRef, useState } from "react";
import type { CSSProperties } from "react";

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
  onSelectionComplete: (pixels: Pixel[]) => void;
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

type SelectionRect = {
  x: number;
  y: number;
  width: number;
  height: number;
};

export default function PixelCanvas({
  width,
  height,
  pixels,
  onPixelClick,
  onSelectionComplete,
}: PixelCanvasProps) {
  const canvasRef = useRef<HTMLCanvasElement | null>(null);
  const [selectionRect, setSelectionRect] = useState<SelectionRect | null>(null);
  const [previewPixels, setPreviewPixels] = useState<Pixel[]>([]);
  const dragStartRef = useRef<{ x: number; y: number } | null>(null);
  const isDraggingRef = useRef(false);
  const didDragRef = useRef(false);
  const preventClickRef = useRef(false);

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

  const getCanvasPosition = (event: MouseEvent<HTMLCanvasElement>) => {
    const canvas = canvasRef.current;
    if (!canvas) {
      return null;
    }
    const rect = canvas.getBoundingClientRect();
    const scaleX = canvas.width / rect.width;
    const scaleY = canvas.height / rect.height;
    const x = Math.floor((event.clientX - rect.left) * scaleX);
    const y = Math.floor((event.clientY - rect.top) * scaleY);
    if (x < 0 || y < 0 || x >= width || y >= height) {
      return null;
    }
    return { x, y, canvas };
  };

  const resetSelection = () => {
    setSelectionRect(null);
    setPreviewPixels([]);
    dragStartRef.current = null;
    isDraggingRef.current = false;
    didDragRef.current = false;
  };

  const handleClick = (event: MouseEvent<HTMLCanvasElement>) => {
    if (preventClickRef.current) {
      preventClickRef.current = false;
      return;
    }

    const position = getCanvasPosition(event);
    if (!position) return;
    const index = position.y * width + position.x;
    const pixel = data[index];
    if (pixel) {
      onPixelClick(pixel);
    }
  };

  const handleMouseDown = (event: MouseEvent<HTMLCanvasElement>) => {
    if (event.button !== 0) return;
    const position = getCanvasPosition(event);
    if (!position) return;
    dragStartRef.current = { x: position.x, y: position.y };
    isDraggingRef.current = true;
    didDragRef.current = false;
    preventClickRef.current = false;
    setSelectionRect(null);
    setPreviewPixels([]);
  };

  const handleMouseMove = (event: MouseEvent<HTMLCanvasElement>) => {
    if (!isDraggingRef.current || !dragStartRef.current) return;
    const position = getCanvasPosition(event);
    if (!position) return;

    const start = dragStartRef.current;
    const minX = Math.min(start.x, position.x);
    const minY = Math.min(start.y, position.y);
    const maxX = Math.max(start.x, position.x);
    const maxY = Math.max(start.y, position.y);

    if (maxX !== minX || maxY !== minY) {
      didDragRef.current = true;
    }

    const rect: SelectionRect = {
      x: minX,
      y: minY,
      width: maxX - minX + 1,
      height: maxY - minY + 1,
    };

    const freePixels: Pixel[] = [];
    for (let py = rect.y; py < rect.y + rect.height; py++) {
      const row = py * width;
      for (let px = rect.x; px < rect.x + rect.width; px++) {
        const pixel = data[row + px];
        if (pixel && pixel.status === "free") {
          freePixels.push(pixel);
        }
      }
    }

    setSelectionRect(rect);
    setPreviewPixels(freePixels);
  };

  const finalizeSelection = () => {
    if (!didDragRef.current) {
      resetSelection();
      return;
    }

    preventClickRef.current = true;
    onSelectionComplete([...previewPixels]);
    resetSelection();
  };

  const handleMouseUp = (event: MouseEvent<HTMLCanvasElement>) => {
    if (event.button !== 0) return;
    if (!isDraggingRef.current) return;
    finalizeSelection();
  };

  const handleMouseLeave = () => {
    if (isDraggingRef.current) {
      preventClickRef.current = true;
    }
    resetSelection();
  };

  return (
    <div className="relative w-full max-w-4xl">
      <canvas
        ref={canvasRef}
        width={width}
        height={height}
        onClick={handleClick}
        onMouseDown={handleMouseDown}
        onMouseMove={handleMouseMove}
        onMouseUp={handleMouseUp}
        onMouseLeave={handleMouseLeave}
        className="w-full border border-slate-700 rounded-lg shadow-md"
        style={{
          imageRendering: "pixelated",
          aspectRatio: `${width} / ${height}`,
          backgroundColor: "#111827"
        }}
      />
      {selectionRect && (
        <div
          className="pointer-events-none absolute border-2 border-blue-400/80 bg-blue-400/10"
          style={(() => {
            const canvas = canvasRef.current;
            if (!canvas) {
              return { display: "none" } as CSSProperties;
            }
            const displayWidth = canvas.clientWidth;
            const displayHeight = canvas.clientHeight;
            return {
              left: `${(selectionRect.x / width) * displayWidth}px`,
              top: `${(selectionRect.y / height) * displayHeight}px`,
              width: `${(selectionRect.width / width) * displayWidth}px`,
              height: `${(selectionRect.height / height) * displayHeight}px`,
            } as CSSProperties;
          })()}
        >
          <span className="sr-only">{previewPixels.length} wolnych pikseli zaznaczonych</span>
        </div>
      )}
    </div>
  );
}
