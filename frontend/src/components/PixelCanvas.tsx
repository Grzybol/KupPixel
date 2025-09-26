import { MouseEvent, PointerEvent, useCallback, useEffect, useMemo, useRef, useState } from "react";
import type { CSSProperties } from "react";
import { useI18n } from "../lang/I18nProvider";

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
type WheelEvent = globalThis.WheelEvent;

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
  const offscreenCanvasRef = useRef<HTMLCanvasElement | null>(null);
  const [selectionRect, setSelectionRect] = useState<SelectionRect | null>(null);
  const [previewPixels, setPreviewPixels] = useState<Pixel[]>([]);
  const [isHovered, setIsHovered] = useState(false);
  const dragStartRef = useRef<{ x: number; y: number } | null>(null);
  const isDraggingRef = useRef(false);
  const didDragRef = useRef(false);
  const preventClickRef = useRef(false);
  const isPanningRef = useRef(false);
  const lastPanPositionRef = useRef<{ x: number; y: number } | null>(null);
  const originalBodyOverflowRef = useRef<string | null>(null);
  const { t } = useI18n();

  const MIN_WINDOW_SIZE = 3;
  const ZOOM_STEP = 1.25;

  const [zoom, setZoom] = useState(1);
  const [offsetX, setOffsetX] = useState(0);
  const [offsetY, setOffsetY] = useState(0);

  const data = useMemo(() => pixels, [pixels]);

  useEffect(() => {
    setZoom(1);
    setOffsetX(0);
    setOffsetY(0);
    setSelectionRect(null);
    setPreviewPixels([]);
  }, [width, height]);

  const clampOffset = useCallback(
    (value: number, visible: number, total: number) => {
      const max = Math.max(0, total - visible);
      if (!Number.isFinite(value)) return 0;
      return Math.min(Math.max(value, 0), max);
    },
    []
  );

  const maxZoom = useMemo(() => {
    const maxWidthZoom = width / MIN_WINDOW_SIZE;
    const maxHeightZoom = height / MIN_WINDOW_SIZE;
    return Math.max(1, Math.min(maxWidthZoom, maxHeightZoom));
  }, [height, width]);

  const computeVisibleWidth = useCallback(
    (currentZoom: number) => {
      const base = Math.round(width / currentZoom);
      return Math.max(MIN_WINDOW_SIZE, Math.min(width, base));
    },
    [width]
  );

  const computeVisibleHeight = useCallback(
    (currentZoom: number) => {
      const base = Math.round(height / currentZoom);
      return Math.max(MIN_WINDOW_SIZE, Math.min(height, base));
    },
    [height]
  );

  const visibleWidth = useMemo(() => computeVisibleWidth(zoom), [computeVisibleWidth, zoom]);
  const visibleHeight = useMemo(() => computeVisibleHeight(zoom), [computeVisibleHeight, zoom]);

  useEffect(() => {
    setOffsetX((prev) => clampOffset(prev, visibleWidth, width));
  }, [clampOffset, visibleWidth, width]);

  useEffect(() => {
    setOffsetY((prev) => clampOffset(prev, visibleHeight, height));
  }, [clampOffset, visibleHeight, height]);

  useEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas) return;
    let offscreen = offscreenCanvasRef.current;
    if (!offscreen) {
      offscreen = document.createElement("canvas");
      offscreenCanvasRef.current = offscreen;
    }
    offscreen.width = width;
    offscreen.height = height;
    const ctx = offscreen.getContext("2d");
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

  useEffect(() => {
    const canvas = canvasRef.current;
    const offscreen = offscreenCanvasRef.current;
    if (!canvas || !offscreen) return;
    const ctx = canvas.getContext("2d");
    if (!ctx) return;
    ctx.imageSmoothingEnabled = false;
    ctx.clearRect(0, 0, canvas.width, canvas.height);
    ctx.drawImage(
      offscreen,
      offsetX,
      offsetY,
      visibleWidth,
      visibleHeight,
      0,
      0,
      canvas.width,
      canvas.height
    );
    const pixelWidth = visibleWidth === 0 ? Infinity : canvas.width / visibleWidth;
    const pixelHeight = visibleHeight === 0 ? Infinity : canvas.height / visibleHeight;
    const pixelScreenSize = Math.min(pixelWidth, pixelHeight);
    if (pixelScreenSize >= 4) {
      ctx.save();
      ctx.imageSmoothingEnabled = false;
      ctx.lineWidth = 1;
      ctx.strokeStyle = "rgba(148, 163, 184, 0.35)";
      ctx.translate(0.5, 0.5);
      ctx.beginPath();
      const maxBoardX = offsetX + visibleWidth;
      for (let x = Math.floor(offsetX) + 1; x < maxBoardX; x++) {
        const canvasX = (x - offsetX) * pixelWidth;
        ctx.moveTo(canvasX, 0);
        ctx.lineTo(canvasX, canvas.height);
      }
      const maxBoardY = offsetY + visibleHeight;
      for (let y = Math.floor(offsetY) + 1; y < maxBoardY; y++) {
        const canvasY = (y - offsetY) * pixelHeight;
        ctx.moveTo(0, canvasY);
        ctx.lineTo(canvas.width, canvasY);
      }
      ctx.stroke();
      ctx.restore();
    }
  }, [offsetX, offsetY, visibleHeight, visibleWidth, data]);

  const getBoardCoordinates = useCallback(
    (event: { clientX: number; clientY: number }) => {
      const canvas = canvasRef.current;
      if (!canvas) {
        return null;
      }
      const rect = canvas.getBoundingClientRect();
      const scaleX = canvas.width / rect.width;
      const scaleY = canvas.height / rect.height;
      const canvasX = (event.clientX - rect.left) * scaleX;
      const canvasY = (event.clientY - rect.top) * scaleY;
      const normalizedX = canvas.width === 0 ? 0 : canvasX / canvas.width;
      const normalizedY = canvas.height === 0 ? 0 : canvasY / canvas.height;
      const boardX = offsetX + normalizedX * visibleWidth;
      const boardY = offsetY + normalizedY * visibleHeight;
      return { canvas, canvasX, canvasY, boardX, boardY };
    },
    [offsetX, offsetY, visibleHeight, visibleWidth]
  );

  const getCanvasPosition = (event: { clientX: number; clientY: number }) => {
    const info = getBoardCoordinates(event);
    if (!info) {
      return null;
    }
    if (info.canvasX < 0 || info.canvasY < 0 || info.canvasX >= info.canvas.width || info.canvasY >= info.canvas.height) {
      return null;
    }
    const x = Math.floor(info.boardX);
    const y = Math.floor(info.boardY);
    if (x < 0 || y < 0 || x >= width || y >= height) {
      return null;
    }
    return { x, y, canvas: info.canvas };
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

  const finalizeSelection = () => {
    if (!didDragRef.current) {
      resetSelection();
      return;
    }

    preventClickRef.current = true;
    onSelectionComplete([...previewPixels]);
    resetSelection();
  };

  const startPanning = useCallback((clientX: number, clientY: number) => {
    isPanningRef.current = true;
    lastPanPositionRef.current = { x: clientX, y: clientY };
  }, []);

  const updatePanning = useCallback(
    (clientX: number, clientY: number, canvas: HTMLCanvasElement) => {
      const last = lastPanPositionRef.current;
      if (!last) {
        lastPanPositionRef.current = { x: clientX, y: clientY };
        return;
      }
      const rect = canvas.getBoundingClientRect();
      const deltaX = clientX - last.x;
      const deltaY = clientY - last.y;
      lastPanPositionRef.current = { x: clientX, y: clientY };
      if (rect.width === 0 || rect.height === 0) {
        return;
      }
      const boardDeltaX = (deltaX / rect.width) * visibleWidth;
      const boardDeltaY = (deltaY / rect.height) * visibleHeight;
      if (boardDeltaX !== 0 || boardDeltaY !== 0) {
        preventClickRef.current = true;
      }
      setOffsetX((prev) => clampOffset(prev - boardDeltaX, visibleWidth, width));
      setOffsetY((prev) => clampOffset(prev - boardDeltaY, visibleHeight, height));
    },
    [clampOffset, height, visibleHeight, visibleWidth, width]
  );

  const handlePointerDown = (event: PointerEvent<HTMLCanvasElement>) => {
    if (event.pointerType === "mouse") {
      if (event.button !== 0) return;
      if (event.ctrlKey || event.shiftKey) {
        startPanning(event.clientX, event.clientY);
        preventClickRef.current = false;
        resetSelection();
        return;
      }
      const position = getCanvasPosition(event);
      if (!position) return;
      dragStartRef.current = { x: position.x, y: position.y };
      isDraggingRef.current = true;
      didDragRef.current = false;
      preventClickRef.current = false;
      setSelectionRect(null);
      setPreviewPixels([]);
      return;
    }

    startPanning(event.clientX, event.clientY);
    preventClickRef.current = false;
    resetSelection();
    event.preventDefault();
    if (event.currentTarget.setPointerCapture) {
      event.currentTarget.setPointerCapture(event.pointerId);
    }
  };

  const handlePointerMove = (event: PointerEvent<HTMLCanvasElement>) => {
    if (event.pointerType === "mouse") {
      if ((event.buttons & 1) === 1 && (event.ctrlKey || event.shiftKey) && !isPanningRef.current) {
        startPanning(event.clientX, event.clientY);
        preventClickRef.current = true;
        resetSelection();
      }
    }

    if (isPanningRef.current) {
      if (event.pointerType !== "mouse") {
        event.preventDefault();
      }
      updatePanning(event.clientX, event.clientY, event.currentTarget);
      return;
    }

    if (event.pointerType !== "mouse") {
      return;
    }

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

  const handlePointerUp = (event: PointerEvent<HTMLCanvasElement>) => {
    if (event.pointerType !== "mouse") {
      if (event.currentTarget.releasePointerCapture) {
        try {
          event.currentTarget.releasePointerCapture(event.pointerId);
        } catch {
          // Ignore errors if the pointer was not captured.
        }
      }
      if (isPanningRef.current) {
        isPanningRef.current = false;
        lastPanPositionRef.current = null;
      }
      return;
    }

    if (event.button !== 0) return;
    if (isPanningRef.current || event.ctrlKey || event.shiftKey) {
      isPanningRef.current = false;
      lastPanPositionRef.current = null;
      preventClickRef.current = true;
      return;
    }
    if (!isDraggingRef.current) return;
    finalizeSelection();
  };

  const handlePointerCancel = (event: PointerEvent<HTMLCanvasElement>) => {
    if (event.currentTarget.releasePointerCapture) {
      try {
        event.currentTarget.releasePointerCapture(event.pointerId);
      } catch {
        // Ignore errors if the pointer was not captured.
      }
    }
    isPanningRef.current = false;
    lastPanPositionRef.current = null;
    preventClickRef.current = true;
    resetSelection();
  };

  const handleMouseEnter = () => {
    setIsHovered(true);
  };

  const handleMouseLeave = () => {
    setIsHovered(false);
    if (isDraggingRef.current || isPanningRef.current) {
      preventClickRef.current = true;
    }
    isPanningRef.current = false;
    lastPanPositionRef.current = null;
    resetSelection();
  };

  const adjustZoom = useCallback(
    (direction: "in" | "out", anchor?: { x: number; y: number }) => {
      setZoom((prevZoom) => {
        const targetZoom = direction === "in" ? prevZoom * ZOOM_STEP : prevZoom / ZOOM_STEP;
        const newZoom = Math.min(maxZoom, Math.max(1, targetZoom));
        const prevVisibleWidth = computeVisibleWidth(prevZoom);
        const prevVisibleHeight = computeVisibleHeight(prevZoom);
        const newVisibleWidth = computeVisibleWidth(newZoom);
        const newVisibleHeight = computeVisibleHeight(newZoom);
        if (anchor) {
          setOffsetX((prevOffset) => {
            const anchorRatioX = prevVisibleWidth > 0 ? (anchor.x - prevOffset) / prevVisibleWidth : 0;
            const nextOffset = anchor.x - anchorRatioX * newVisibleWidth;
            return clampOffset(nextOffset, newVisibleWidth, width);
          });
          setOffsetY((prevOffset) => {
            const anchorRatioY = prevVisibleHeight > 0 ? (anchor.y - prevOffset) / prevVisibleHeight : 0;
            const nextOffset = anchor.y - anchorRatioY * newVisibleHeight;
            return clampOffset(nextOffset, newVisibleHeight, height);
          });
        } else {
          setOffsetX((prevOffset) => {
            const center = prevOffset + prevVisibleWidth / 2;
            const nextOffset = center - newVisibleWidth / 2;
            return clampOffset(nextOffset, newVisibleWidth, width);
          });
          setOffsetY((prevOffset) => {
            const center = prevOffset + prevVisibleHeight / 2;
            const nextOffset = center - newVisibleHeight / 2;
            return clampOffset(nextOffset, newVisibleHeight, height);
          });
        }
        return newZoom;
      });
    },
    [clampOffset, computeVisibleHeight, computeVisibleWidth, height, maxZoom, width]
  );

  const handleWheel = useCallback(
    (event: WheelEvent) => {
      event.preventDefault();
      event.stopPropagation();
      const info = getBoardCoordinates(event);
      const anchor = info
        ? {
            x: Math.min(Math.max(info.boardX, 0), Math.max(0, width - 1)),
            y: Math.min(Math.max(info.boardY, 0), Math.max(0, height - 1)),
          }
        : undefined;
      if (event.deltaY > 0) {
        adjustZoom("out", anchor);
      } else if (event.deltaY < 0) {
        adjustZoom("in", anchor);
      }
    },
    [adjustZoom, getBoardCoordinates, height, width]
  );

  useEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas) return;

    canvas.addEventListener("wheel", handleWheel, { passive: false });
    return () => {
      canvas.removeEventListener("wheel", handleWheel);
    };
  }, [handleWheel]);

  useEffect(() => {
    const body = document.body;
    if (!body) return;

    if (isHovered) {
      if (originalBodyOverflowRef.current === null) {
        originalBodyOverflowRef.current = body.style.overflow;
      }
      body.style.overflow = "hidden";
    } else if (originalBodyOverflowRef.current !== null) {
      body.style.overflow = originalBodyOverflowRef.current;
      originalBodyOverflowRef.current = null;
    }

    return () => {
      if (originalBodyOverflowRef.current !== null) {
        body.style.overflow = originalBodyOverflowRef.current;
        originalBodyOverflowRef.current = null;
      }
    };
  }, [isHovered]);

  const EPSILON = 1e-6;
  const canZoomIn = zoom < maxZoom - EPSILON && (visibleWidth > MIN_WINDOW_SIZE || visibleHeight > MIN_WINDOW_SIZE);
  const canZoomOut = zoom > 1 + EPSILON;

  return (
    <div className="relative w-full max-w-4xl">
      <canvas
        ref={canvasRef}
        width={width}
        height={height}
        onClick={handleClick}
        onPointerDown={handlePointerDown}
        onPointerMove={handlePointerMove}
        onPointerUp={handlePointerUp}
        onPointerCancel={handlePointerCancel}
        onMouseEnter={handleMouseEnter}
        onMouseLeave={handleMouseLeave}
        className="w-full border border-slate-700 rounded-lg shadow-md"
        style={{
          imageRendering: "pixelated",
          aspectRatio: `${width} / ${height}`,
          backgroundColor: "#111827",
          touchAction: "none",
          overscrollBehavior: "contain",
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
            const relativeLeft = (selectionRect.x - offsetX) / visibleWidth;
            const relativeTop = (selectionRect.y - offsetY) / visibleHeight;
            const relativeRight = (selectionRect.x + selectionRect.width - offsetX) / visibleWidth;
            const relativeBottom = (selectionRect.y + selectionRect.height - offsetY) / visibleHeight;
            if (
              relativeRight <= 0 ||
              relativeBottom <= 0 ||
              relativeLeft >= 1 ||
              relativeTop >= 1
            ) {
              return { display: "none" } as CSSProperties;
            }
            const clampedLeft = Math.max(0, relativeLeft);
            const clampedTop = Math.max(0, relativeTop);
            const clampedRight = Math.min(1, relativeRight);
            const clampedBottom = Math.min(1, relativeBottom);
            const widthRatio = Math.max(0, clampedRight - clampedLeft);
            const heightRatio = Math.max(0, clampedBottom - clampedTop);
            return {
              left: `${clampedLeft * displayWidth}px`,
              top: `${clampedTop * displayHeight}px`,
              width: `${widthRatio * displayWidth}px`,
              height: `${heightRatio * displayHeight}px`,
            } as CSSProperties;
          })()}
        >
          <span className="sr-only">{t("pixelCanvas.selection", { count: previewPixels.length })}</span>
        </div>
      )}
      <div className="mt-3 flex justify-center gap-3">
        <button
          type="button"
          onClick={() => adjustZoom("out")}
          disabled={!canZoomOut}
          className="rounded-md border border-slate-600 bg-slate-800 px-3 py-1 text-sm font-semibold text-slate-200 disabled:cursor-not-allowed disabled:border-slate-700 disabled:bg-slate-900 disabled:text-slate-500"
        >
          −
        </button>
        <button
          type="button"
          onClick={() => adjustZoom("in")}
          disabled={!canZoomIn}
          className="rounded-md border border-slate-600 bg-slate-800 px-3 py-1 text-sm font-semibold text-slate-200 disabled:cursor-not-allowed disabled:border-slate-700 disabled:bg-slate-900 disabled:text-slate-500"
        >
          +
        </button>
      </div>
    </div>
  );
}
