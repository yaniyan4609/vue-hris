<script setup>
import { ref, computed, onMounted, watch } from 'vue'
import { Plus, Trash2, MapPin, ZoomIn, ZoomOut, RotateCcw, Check, X } from 'lucide-vue-next'
import BaseButton from './BaseButton.vue'
import BaseBadge from './BaseBadge.vue'

const props = defineProps({
  modelValue: {
    type: Array,
    default: () => [
      // Example default polygon
      [
        { lat: -6.2088, lng: 106.8456 },
        { lat: -6.2088, lng: 106.8476 },
        { lat: -6.2104, lng: 106.8476 },
        { lat: -6.2104, lng: 106.8456 },
      ],
    ],
  },
  center: {
    type: Object,
    default: () => ({ lat: -6.2096, lng: 106.8466 }),
  },
  zoom: {
    type: Number,
    default: 16,
  },
  editable: {
    type: Boolean,
    default: true,
  },
  height: {
    type: String,
    default: '360px',
  },
  label: {
    type: String,
    default: 'Area Geofence Multi-Polygon',
  },
})

const emit = defineEmits(['update:modelValue', 'change'])

const currentCenter = ref({ ...props.center })
const currentZoom = ref(props.zoom)
const activePolygonIndex = ref(0)
const selectedVertex = ref(null) // { polyIdx, vertIdx }
const draggingVertex = ref(null)
const mapSvgRef = ref(null)
const testCoord = ref({ lat: -6.2096, lng: 106.8466 })
const testResult = ref(null)

// Web Mercator projection formulas (WGS84 lat/lng to/from pixel at given zoom)
const TILE_SIZE = 256

const latLngToWorldPixel = (lat, lng, zoom) => {
  const scale = 1 << zoom
  const x = ((lng + 180) / 360) * scale * TILE_SIZE
  const sinY = Math.sin((lat * Math.PI) / 180)
  const clippedSinY = Math.min(Math.max(sinY, -0.9999), 0.9999)
  const y = (0.5 - Math.log((1 + clippedSinY) / (1 - clippedSinY)) / (4 * Math.PI)) * scale * TILE_SIZE
  return { x, y }
}

const worldPixelToLatLng = (x, y, zoom) => {
  const scale = 1 << zoom
  const lng = (x / (scale * TILE_SIZE)) * 360 - 180
  const n = Math.PI - (2 * Math.PI * y) / (scale * TILE_SIZE)
  const lat = (180 / Math.PI) * Math.atan(0.5 * (Math.exp(n) - Math.exp(-n)))
  return { lat, lng }
}

// Convert lat/lng to container SVG coordinates relative to map center (cx=300, cy=180)
const viewWidth = 600
const viewHeight = 360

const centerPixel = computed(() => {
  return latLngToWorldPixel(currentCenter.value.lat, currentCenter.value.lng, currentZoom.value)
})

const toSvgCoord = (lat, lng) => {
  const pt = latLngToWorldPixel(lat, lng, currentZoom.value)
  return {
    x: pt.x - centerPixel.value.x + viewWidth / 2,
    y: pt.y - centerPixel.value.y + viewHeight / 2,
  }
}

const fromSvgCoord = (svgX, svgY) => {
  const worldX = svgX - viewWidth / 2 + centerPixel.value.x
  const worldY = svgY - viewHeight / 2 + centerPixel.value.y
  return worldPixelToLatLng(worldX, worldY, currentZoom.value)
}

// Compute visible OSM tile grid (3x3 grid around center)
const visibleTiles = computed(() => {
  const scale = 1 << currentZoom.value
  const centerTileX = Math.floor(centerPixel.value.x / TILE_SIZE)
  const centerTileY = Math.floor(centerPixel.value.y / TILE_SIZE)
  const tiles = []

  for (let dx = -2; dx <= 2; dx++) {
    for (let dy = -1; dy <= 1; dy++) {
      const tileX = centerTileX + dx
      const tileY = centerTileY + dy
      if (tileX >= 0 && tileX < scale && tileY >= 0 && tileY < scale) {
        const left = tileX * TILE_SIZE - centerPixel.value.x + viewWidth / 2
        const top = tileY * TILE_SIZE - centerPixel.value.y + viewHeight / 2
        tiles.push({
          x: tileX,
          y: tileY,
          z: currentZoom.value,
          left,
          top,
          url: `https://tile.openstreetmap.org/${currentZoom.value}/${tileX}/${tileY}.png`,
        })
      }
    }
  }
  return tiles
})

// Calculate polygon area in square meters using Earth radius 6378137m
const calculatePolygonArea = (poly) => {
  if (!poly || poly.length < 3) return 0
  const R = 6378137
  let area = 0
  for (let i = 0; i < poly.length; i++) {
    const j = (i + 1) % poly.length
    const p1 = poly[i]
    const p2 = poly[j]
    const lat1 = (p1.lat * Math.PI) / 180
    const lat2 = (p2.lat * Math.PI) / 180
    const dLng = ((p2.lng - p1.lng) * Math.PI) / 180
    area += dLng * (2 + Math.sin(lat1) + Math.sin(lat2))
  }
  return Math.abs(Math.round((area * R * R) / 4))
}

const totalArea = computed(() => {
  return props.modelValue.reduce((acc, poly) => acc + calculatePolygonArea(poly), 0)
})

// Point in polygon test (Ray-casting algorithm)
const isPointInPoly = (pt, poly) => {
  let inside = false
  for (let i = 0, j = poly.length - 1; i < poly.length; j = i++) {
    const xi = poly[i].lng, yi = poly[i].lat
    const xj = poly[j].lng, yj = poly[j].lat
    const intersect = yi > pt.lat !== yj > pt.lat && pt.lng < ((xj - xi) * (pt.lat - yi)) / (yj - yi) + xi
    if (intersect) inside = !inside
  }
  return inside
}

const runPointTest = () => {
  const isInside = props.modelValue.some((poly) => isPointInPoly(testCoord.value, poly))
  testResult.value = isInside
}

// Interaction: click on map canvas
const handleSvgClick = (e) => {
  if (!props.editable || draggingVertex.value) return
  const rect = mapSvgRef.value.getBoundingClientRect()
  const svgX = (e.clientX - rect.left) * (viewWidth / rect.width)
  const svgY = (e.clientY - rect.top) * (viewHeight / rect.height)
  const coord = fromSvgCoord(svgX, svgY)

  const next = JSON.parse(JSON.stringify(props.modelValue))
  if (!next[activePolygonIndex.value]) {
    next[activePolygonIndex.value] = []
  }
  next[activePolygonIndex.value].push({
    lat: Number(coord.lat.toFixed(6)),
    lng: Number(coord.lng.toFixed(6)),
  })

  emit('update:modelValue', next)
  emit('change', next)
}

// Start dragging vertex
const startDragVertex = (polyIdx, vertIdx, e) => {
  e.stopPropagation()
  if (!props.editable) return
  draggingVertex.value = { polyIdx, vertIdx }
  selectedVertex.value = { polyIdx, vertIdx }

  const onMouseMove = (ev) => {
    if (!draggingVertex.value || !mapSvgRef.value) return
    const rect = mapSvgRef.value.getBoundingClientRect()
    const svgX = (ev.clientX - rect.left) * (viewWidth / rect.width)
    const svgY = (ev.clientY - rect.top) * (viewHeight / rect.height)
    const coord = fromSvgCoord(svgX, svgY)

    const next = JSON.parse(JSON.stringify(props.modelValue))
    next[polyIdx][vertIdx] = {
      lat: Number(coord.lat.toFixed(6)),
      lng: Number(coord.lng.toFixed(6)),
    }
    emit('update:modelValue', next)
  }

  const onMouseUp = () => {
    window.removeEventListener('mousemove', onMouseMove)
    window.removeEventListener('mouseup', onMouseUp)
    draggingVertex.value = null
    emit('change', props.modelValue)
  }

  window.addEventListener('mousemove', onMouseMove)
  window.addEventListener('mouseup', onMouseUp)
}

const deleteSelectedVertex = () => {
  if (!selectedVertex.value) return
  const { polyIdx, vertIdx } = selectedVertex.value
  const next = JSON.parse(JSON.stringify(props.modelValue))
  if (next[polyIdx]) {
    next[polyIdx].splice(vertIdx, 1)
    if (next[polyIdx].length === 0 && next.length > 1) {
      next.splice(polyIdx, 1)
      activePolygonIndex.value = Math.max(0, activePolygonIndex.value - 1)
    }
  }
  selectedVertex.value = null
  emit('update:modelValue', next)
  emit('change', next)
}

const addNewPolygon = () => {
  const next = JSON.parse(JSON.stringify(props.modelValue))
  next.push([])
  activePolygonIndex.value = next.length - 1
  emit('update:modelValue', next)
}

const clearAll = () => {
  emit('update:modelValue', [[]])
  emit('change', [[]])
  selectedVertex.value = null
}

const zoomIn = () => {
  if (currentZoom.value < 19) currentZoom.value++
}

const zoomOut = () => {
  if (currentZoom.value > 12) currentZoom.value--
}

const resetCenter = () => {
  currentCenter.value = { ...props.center }
  currentZoom.value = props.zoom
}
</script>

<template>
  <div class="w-full space-y-2.5 select-none">
    <!-- Top Bar: Judul, Status & Action Toolbar -->
    <div class="flex flex-wrap items-center justify-between gap-2 text-xs">
      <div class="flex items-center gap-2">
        <span class="font-bold text-neutral-800 dark:text-neutral-200">{{ label }}</span>
        <BaseBadge variant="info">
          {{ modelValue.length }} Poligon
        </BaseBadge>
        <span class="text-[11px] font-mono text-neutral-500 dark:text-neutral-400">
          Est. Luas: <strong>{{ totalArea.toLocaleString('id-ID') }} m²</strong>
        </span>
      </div>

      <div v-if="editable" class="flex items-center gap-1.5 flex-wrap">
        <BaseButton size="sm" variant="outline" @click="addNewPolygon">
          <Plus class="w-3.5 h-3.5" />
          <span>Poligon Baru</span>
        </BaseButton>

        <BaseButton
          v-if="selectedVertex"
          size="sm"
          variant="danger"
          @click="deleteSelectedVertex"
        >
          <Trash2 class="w-3.5 h-3.5" />
          <span>Hapus Titik</span>
        </BaseButton>

        <BaseButton size="sm" variant="ghost" @click="clearAll">
          Bersihkan
        </BaseButton>
      </div>
    </div>

    <!-- Map Canvas Container -->
    <div
      class="relative w-full rounded-2xl border border-neutral-300 dark:border-[var(--theme-border)] overflow-hidden bg-neutral-100 dark:bg-[var(--theme-layer-main)] shadow-inner"
      :style="{ height }"
    >
      <!-- Layer 1: OpenStreetMap Tiles -->
      <div class="absolute inset-0 pointer-events-none overflow-hidden opacity-90 dark:opacity-75 dark:invert dark:hue-rotate-180">
        <img
          v-for="t in visibleTiles"
          :key="`${t.z}-${t.x}-${t.y}`"
          :src="t.url"
          alt="Map Tile"
          class="absolute w-[256px] h-[256px] select-none pointer-events-none"
          :style="{ left: `${t.left}px`, top: `${t.top}px` }"
          loading="lazy"
        />
      </div>

      <!-- Layer 2: SVG Vector Geometry (Polygons, Lines, Vertices) -->
      <svg
        ref="mapSvgRef"
        :viewBox="`0 0 ${viewWidth} ${viewHeight}`"
        class="absolute inset-0 w-full h-full cursor-crosshair"
        @click="handleSvgClick"
      >
        <g v-for="(poly, pIdx) in modelValue" :key="pIdx">
          <!-- Polygon Fill & Stroke -->
          <polygon
            v-if="poly.length >= 3"
            :points="poly.map(pt => `${toSvgCoord(pt.lat, pt.lng).x},${toSvgCoord(pt.lat, pt.lng).y}`).join(' ')"
            class="transition-all"
            :class="
              pIdx === activePolygonIndex
                ? 'fill-[var(--theme-primary)]/25 stroke-[var(--theme-primary)] stroke-2'
                : 'fill-emerald-500/20 stroke-emerald-600 stroke-2'
            "
          />

          <!-- Open Polyline while drawing -->
          <polyline
            v-else-if="poly.length === 2"
            :points="poly.map(pt => `${toSvgCoord(pt.lat, pt.lng).x},${toSvgCoord(pt.lat, pt.lng).y}`).join(' ')"
            class="stroke-[var(--theme-primary)] stroke-2 stroke-dashed"
          />

          <!-- Vertex Points -->
          <circle
            v-for="(pt, vIdx) in poly"
            :key="`${pIdx}-${vIdx}`"
            :cx="toSvgCoord(pt.lat, pt.lng).x"
            :cy="toSvgCoord(pt.lat, pt.lng).y"
            r="6"
            class="cursor-grab active:cursor-grabbing transition-transform hover:scale-125 shadow-md"
            :class="
              selectedVertex?.polyIdx === pIdx && selectedVertex?.vertIdx === vIdx
                ? 'fill-rose-500 stroke-white stroke-2'
                : 'fill-[var(--theme-primary)] stroke-white stroke-2'
            "
            @mousedown="startDragVertex(pIdx, vIdx, $event)"
          />
        </g>
      </svg>

      <!-- Map Floating Controls (Zoom & Recenter) -->
      <div class="absolute top-3 right-3 flex flex-col gap-1 z-20">
        <button
          type="button"
          @click="zoomIn"
          class="w-7 h-7 rounded-lg bg-white/95 dark:bg-[var(--theme-surface-raised)]/95 border border-neutral-300 dark:border-[var(--theme-border-strong)] flex items-center justify-center text-neutral-700 dark:text-neutral-200 shadow-sm hover:bg-neutral-100 transition cursor-pointer"
          title="Zoom In"
        >
          <ZoomIn class="w-3.5 h-3.5" />
        </button>
        <button
          type="button"
          @click="zoomOut"
          class="w-7 h-7 rounded-lg bg-white/95 dark:bg-[var(--theme-surface-raised)]/95 border border-neutral-300 dark:border-[var(--theme-border-strong)] flex items-center justify-center text-neutral-700 dark:text-neutral-200 shadow-sm hover:bg-neutral-100 transition cursor-pointer"
          title="Zoom Out"
        >
          <ZoomOut class="w-3.5 h-3.5" />
        </button>
        <button
          type="button"
          @click="resetCenter"
          class="w-7 h-7 rounded-lg bg-white/95 dark:bg-[var(--theme-surface-raised)]/95 border border-neutral-300 dark:border-[var(--theme-border-strong)] flex items-center justify-center text-neutral-700 dark:text-neutral-200 shadow-sm hover:bg-neutral-100 transition cursor-pointer"
          title="Tengahkan Peta"
        >
          <RotateCcw class="w-3.5 h-3.5" />
        </button>
      </div>

      <!-- Instruction Overlay -->
      <div class="absolute bottom-2 left-2 z-20 px-2.5 py-1 rounded-xl bg-black/65 backdrop-blur-xs text-[10.5px] text-white flex items-center gap-2">
        <MapPin class="w-3 h-3 text-emerald-400 shrink-0" />
        <span>Klik untuk tambah titik, geser titik untuk ubah bentuk geofence.</span>
      </div>
    </div>

    <!-- Geofence Simulation Tester -->
    <div class="p-3 rounded-2xl bg-neutral-50 dark:bg-[var(--theme-surface-raised)] border border-neutral-200/80 dark:border-[var(--theme-border)] flex flex-wrap items-center justify-between gap-2.5 text-xs">
      <div class="flex items-center gap-2">
        <span class="font-semibold text-neutral-700 dark:text-neutral-300">Uji Lokasi Presensi:</span>
        <span class="font-mono text-neutral-500">Lat: {{ testCoord.lat }}, Lng: {{ testCoord.lng }}</span>
      </div>
      <div class="flex items-center gap-2">
        <BaseButton size="sm" variant="secondary" @click="runPointTest">
          Cek Dalam Geofence
        </BaseButton>
        <BaseBadge
          v-if="testResult !== null"
          :variant="testResult ? 'success' : 'danger'"
        >
          <component :is="testResult ? Check : X" class="w-3 h-3 mr-0.5 inline" />
          {{ testResult ? 'Di Dalam Radius' : 'Di Luar Radius' }}
        </BaseBadge>
      </div>
    </div>
  </div>
</template>

