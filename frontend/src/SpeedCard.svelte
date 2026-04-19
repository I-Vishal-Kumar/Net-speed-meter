<script lang="ts">
  import { speed } from './stores/speed'
  import { formatSpeed } from './lib/format'
  import { StartDrag, ExitCalibrate } from '../wailsjs/go/main/App'
  import { EventsOn } from '../wailsjs/runtime/runtime'

  $: dl = formatSpeed($speed.Download)
  $: ul = formatSpeed($speed.Upload)

  // ── Calibrate state ──────────────────────────────────────────────────
  let calibrating = false
  let dpr = 1
  // Our window covers the work area. These are in CSS px, relative to that window.
  let workW = 0
  let workH = 0
  // Origin of the work area in physical screen coords — added back on save.
  let workOrigin = { x: 0, y: 0 }

  // box = the dashed outline. The live widget is pinned on top of it, so
  // dragging the outline moves the widget and resizing it resizes the widget.
  let box = { x: 0, y: 0, w: 0, h: 0 }

  $: liveFont = Math.max(8, Math.round(box.h * 0.22))

  type Mode = null | 'move' | 'n' | 's' | 'e' | 'w' | 'ne' | 'nw' | 'se' | 'sw'
  let dragMode: Mode = null
  let dragStart = { mx: 0, my: 0, x: 0, y: 0, w: 0, h: 0 }

  const MIN_W = 60
  const MIN_H = 20

  EventsOn('calibrate:enter', (p: any) => {
    dpr = window.devicePixelRatio || 1
    workOrigin = { x: p.workX, y: p.workY }
    workW = p.workW / dpr
    workH = p.workH / dpr

    // Translate the widget's current screen position into our window's CSS
    // coords. If the widget lives in the taskbar (outside the work area) it
    // clamps to the bottom-right of the work area, which is visually right
    // above where it normally sits.
    const curW = Math.round(p.currentW / dpr)
    const curH = Math.round(p.currentH / dpr)
    const curWinX = Math.round((p.currentX - p.workX) / dpr)
    const curWinY = Math.round((p.currentY - p.workY) / dpr)

    let bx = Math.max(0, Math.min(workW - curW, curWinX))
    let by = Math.max(0, Math.min(workH - curH, curWinY))

    // If the widget is tiny, start with a more useful initial size so the
    // user has something to grab.
    let bw = curW
    let bh = curH
    if (curW < 120 || curH < 40) {
      bw = Math.max(curW * 3, 200)
      bh = Math.max(curH * 3, 80)
      bx = Math.max(0, Math.min(workW - bw, workW - bw))
      by = Math.max(0, Math.min(workH - bh, workH - bh))
    }
    box = { x: bx, y: by, w: bw, h: bh }
    calibrating = true
  })
  EventsOn('calibrate:exit', () => { calibrating = false })

  function onBarMouseDown(e: MouseEvent) {
    if (calibrating) return
    if (e.button === 0) StartDrag()
  }

  function startDrag(mode: Mode, e: MouseEvent) {
    e.preventDefault()
    e.stopPropagation()
    dragMode = mode
    dragStart = { mx: e.screenX, my: e.screenY, x: box.x, y: box.y, w: box.w, h: box.h }
    window.addEventListener('mousemove', onMove)
    window.addEventListener('mouseup', onUp)
  }

  function onMove(e: MouseEvent) {
    if (!dragMode) return
    const dx = e.screenX - dragStart.mx
    const dy = e.screenY - dragStart.my
    let { x, y, w, h } = dragStart
    const m = dragMode
    if (m === 'move') {
      x += dx; y += dy
    } else {
      if (m.includes('e')) w = Math.max(MIN_W, dragStart.w + dx)
      if (m.includes('w')) {
        const nw = Math.max(MIN_W, dragStart.w - dx)
        x = dragStart.x + (dragStart.w - nw)
        w = nw
      }
      if (m.includes('s')) h = Math.max(MIN_H, dragStart.h + dy)
      if (m.includes('n')) {
        const nh = Math.max(MIN_H, dragStart.h - dy)
        y = dragStart.y + (dragStart.h - nh)
        h = nh
      }
    }
    x = Math.max(0, Math.min(workW - w, x))
    y = Math.max(0, Math.min(workH - h, y))
    box = { x, y, w, h }
  }

  function onUp() {
    dragMode = null
    window.removeEventListener('mousemove', onMove)
    window.removeEventListener('mouseup', onUp)
  }

  function onSave() {
    const physX = Math.round(box.x * dpr) + workOrigin.x
    const physY = Math.round(box.y * dpr) + workOrigin.y
    const physW = Math.round(box.w * dpr)
    const physH = Math.round(box.h * dpr)
    ExitCalibrate(physX, physY, physW, physH, true)
  }
  function onCancel() {
    ExitCalibrate(0, 0, 0, 0, false)
  }
  function onKey(e: KeyboardEvent) {
    if (!calibrating) return
    if (e.key === 'Escape') onCancel()
    if (e.key === 'Enter') onSave()
  }
</script>

<svelte:window on:keydown={onKey} />

{#if !calibrating}
  <!-- svelte-ignore a11y-no-static-element-interactions -->
  <div class="bar" on:contextmenu|preventDefault on:mousedown={onBarMouseDown}>
    <div class="row">
      <span class="arrow dl">▼</span>
      <span class="num">{dl.value}</span>
      <span class="unit">{dl.unit}</span>
    </div>
    <div class="row">
      <span class="arrow ul">▲</span>
      <span class="num">{ul.value}</span>
      <span class="unit">{ul.unit}</span>
    </div>
  </div>
{:else}
  <div class="cal-root">
    <!-- Live widget preview: tracks the outline's position AND size, so the
         user sees exactly where and how big the widget will be after saving. -->
    <div
      class="live"
      style="left:{box.x}px;top:{box.y}px;width:{box.w}px;height:{box.h}px;font-size:{liveFont}px"
    >
      <div class="p-row">
        <span class="arrow dl">▼</span>
        <span class="num">{dl.value}</span>
        <span class="unit">{dl.unit}</span>
      </div>
      <div class="p-row">
        <span class="arrow ul">▲</span>
        <span class="num">{ul.value}</span>
        <span class="unit">{ul.unit}</span>
      </div>
    </div>

    <!-- Dashed outline: wraps the live box with a small gap on all sides
         so the dashed border and handles stay visible. Dragging moves the
         outline (and the live widget follows); the handles resize both. -->
    <!-- svelte-ignore a11y-click-events-have-key-events -->
    <div
      class="outline"
      style="left:{box.x - 6}px;top:{box.y - 6}px;width:{box.w + 12}px;height:{box.h + 12}px"
      on:mousedown={(e) => startDrag('move', e)}
    >
      <div class="dim">{box.w} × {box.h}</div>

      <div class="h n"  on:mousedown={(e) => startDrag('n',  e)}></div>
      <div class="h s"  on:mousedown={(e) => startDrag('s',  e)}></div>
      <div class="h e"  on:mousedown={(e) => startDrag('e',  e)}></div>
      <div class="h w"  on:mousedown={(e) => startDrag('w',  e)}></div>
      <div class="h ne" on:mousedown={(e) => startDrag('ne', e)}></div>
      <div class="h nw" on:mousedown={(e) => startDrag('nw', e)}></div>
      <div class="h se" on:mousedown={(e) => startDrag('se', e)}></div>
      <div class="h sw" on:mousedown={(e) => startDrag('sw', e)}></div>
    </div>

    <div class="toolbar">
      <div class="title">Calibrate widget size</div>
      <div class="hint">Drag the dashed outline to resize. The live widget follows. <kbd>Enter</kbd> to save · <kbd>Esc</kbd> to cancel.</div>
      <div class="actions">
        <button class="btn ghost" on:mousedown|stopPropagation on:click={onCancel}>Cancel</button>
        <button class="btn primary" on:mousedown|stopPropagation on:click={onSave}>Save size</button>
      </div>
    </div>
  </div>
{/if}

<style>
  /* ── Normal widget bar ───────────────────────────────────────────────── */
  .bar {
    width: 100vw;
    height: 100vh;
    font-size: 22vh;
    display: flex;
    flex-direction: column;
    justify-content: center;
    align-items: stretch;
    background: transparent;
    cursor: default;
    overflow: hidden;
    padding: 0 0.25em;
    gap: 0.3em;
    box-sizing: border-box;
  }
  .row,
  .p-row {
    display: flex;
    align-items: center;
    gap: 0.15em;
    line-height: 1.05;
    width: 100%;
    justify-content: flex-end;
  }
  .arrow { font-size: 0.55em; width: 1em; text-align: center; flex-shrink: 0; }
  .arrow.dl { color: #4cc2ff; }
  .arrow.ul { color: #ff6b6b; }
  .num {
    font-size: 0.95em;
    font-weight: 600;
    color: #ffffff;
    font-variant-numeric: tabular-nums;
    min-width: 2.4em;
    text-align: right;
    letter-spacing: -0.02em;
  }
  .unit {
    font-size: 0.7em;
    font-weight: 400;
    color: rgba(255, 255, 255, 0.6);
    letter-spacing: 0.02em;
    width: 2em;
    margin-left: 0.1em;
  }

  /* ── Calibrate overlay ───────────────────────────────────────────────── */
  .cal-root {
    position: fixed;
    inset: 0;
    background: rgba(10, 12, 18, 0.55);
    backdrop-filter: blur(2px);
    z-index: 1000;
    font-family: -apple-system, BlinkMacSystemFont, "Segoe UI Variable", "Segoe UI", system-ui, sans-serif;
    color: #fff;
  }

  /* Live widget preview — the solid content box pinned to the taskbar. */
  .live {
    position: absolute;
    box-sizing: border-box;
    display: flex;
    flex-direction: column;
    justify-content: center;
    padding: 0 0.25em;
    gap: 0.3em;
    background: rgba(20, 22, 28, 0.94);
    border: 1.5px solid rgba(255, 255, 255, 0.9);
    border-radius: 4px;
    overflow: hidden;
    pointer-events: none;
  }

  /* Dashed target outline — the user's resize tool. Transparent inside. */
  .outline {
    position: absolute;
    box-sizing: border-box;
    background: transparent;
    border: 1.5px dashed rgba(76, 194, 255, 0.95);
    border-radius: 6px;
    box-shadow: 0 0 0 3px rgba(76, 194, 255, 0.14);
    cursor: move;
  }

  /* Dimension readout floats above the outline's top-right corner. */
  .dim {
    position: absolute;
    top: -24px;
    right: 0;
    padding: 3px 9px;
    font-size: 11px;
    font-weight: 600;
    font-variant-numeric: tabular-nums;
    color: #0b0d12;
    background: #4cc2ff;
    border-radius: 4px;
    pointer-events: none;
    white-space: nowrap;
    box-shadow: 0 2px 6px rgba(0, 0, 0, 0.4);
  }

  /* Resize handles — white pills (edges) and squares (corners). */
  .h {
    position: absolute;
    background: #fff;
    border: 1px solid rgba(0, 0, 0, 0.4);
    border-radius: 2px;
    box-shadow: 0 1px 3px rgba(0, 0, 0, 0.4);
  }
  .h.n  { top: -4px;   left: 50%; transform: translateX(-50%); width: 22px; height: 6px; cursor: ns-resize; border-radius: 3px; }
  .h.s  { bottom:-4px; left: 50%; transform: translateX(-50%); width: 22px; height: 6px; cursor: ns-resize; border-radius: 3px; }
  .h.e  { right: -4px; top: 50%;  transform: translateY(-50%); width: 6px;  height:22px; cursor: ew-resize; border-radius: 3px; }
  .h.w  { left:  -4px; top: 50%;  transform: translateY(-50%); width: 6px;  height:22px; cursor: ew-resize; border-radius: 3px; }
  .h.nw { top: -5px;   left:  -5px; width: 10px; height: 10px; cursor: nwse-resize; }
  .h.ne { top: -5px;   right: -5px; width: 10px; height: 10px; cursor: nesw-resize; }
  .h.sw { bottom:-5px; left:  -5px; width: 10px; height: 10px; cursor: nesw-resize; }
  .h.se { bottom:-5px; right: -5px; width: 10px; height: 10px; cursor: nwse-resize; }

  /* ── Toolbar ─────────────────────────────────────────────────────────── */
  .toolbar {
    position: absolute;
    left: 50%;
    top: 24px;
    transform: translateX(-50%);
    max-width: 560px;
    padding: 14px 18px;
    display: grid;
    grid-template-columns: 1fr auto;
    grid-template-rows: auto auto;
    grid-template-areas:
      "title actions"
      "hint  actions";
    column-gap: 24px;
    row-gap: 4px;
    align-items: center;
    background: rgba(22, 24, 30, 0.95);
    border: 1px solid rgba(255, 255, 255, 0.08);
    border-radius: 10px;
    box-shadow: 0 10px 30px rgba(0, 0, 0, 0.5);
  }
  .title   { grid-area: title;   font-size: 13px; font-weight: 600; letter-spacing: 0.02em; }
  .hint    { grid-area: hint;    font-size: 11.5px; color: rgba(255,255,255,0.55); line-height: 1.4; }
  .actions { grid-area: actions; display: flex; gap: 8px; }

  kbd {
    display: inline-block;
    padding: 1px 6px;
    font-family: inherit;
    font-size: 10.5px;
    font-weight: 600;
    color: rgba(255, 255, 255, 0.85);
    background: rgba(255, 255, 255, 0.08);
    border: 1px solid rgba(255, 255, 255, 0.12);
    border-radius: 4px;
  }

  .btn {
    padding: 7px 14px;
    border: none;
    border-radius: 6px;
    font: inherit;
    font-size: 12.5px;
    font-weight: 600;
    cursor: pointer;
    transition: background 0.12s ease, transform 0.05s ease;
  }
  .btn:active { transform: translateY(1px); }
  .btn.primary {
    background: #4cc2ff;
    color: #0b0d12;
  }
  .btn.primary:hover { background: #6ad0ff; }
  .btn.ghost {
    background: rgba(255, 255, 255, 0.06);
    color: #fff;
  }
  .btn.ghost:hover { background: rgba(255, 255, 255, 0.12); }
</style>
