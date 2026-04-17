<script lang="ts">
  import { speed } from './stores/speed'
  import { formatSpeed } from './lib/format'
  import { StartDrag } from '../wailsjs/go/main/App'

  $: dl = formatSpeed($speed.Download)
  $: ul = formatSpeed($speed.Upload)

  function onMouseDown(e: MouseEvent) {
    if (e.button === 0) {
      StartDrag()
    }
  }
</script>

<!-- svelte-ignore a11y-no-static-element-interactions -->
<div class="bar" on:contextmenu|preventDefault on:mousedown={onMouseDown}>
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

<style>
  /* All sizing is relative to viewport height so the layout fills whatever
     window size the backend sets based on the live taskbar height. */
  .bar {
    width: 100vw;
    height: 100vh;
    /* 1em ≈ 22% of bar height — leaves vertical breathing room above and
       below the two rows so the content sits centered like native tray items. */
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
  }

  .row {
    display: flex;
    align-items: center;
    gap: 0.15em;
    line-height: 1.05;
    width: 100%;
    justify-content: flex-end;
  }


  .arrow {
    font-size: 0.55em;
    width: 1em;
    text-align: center;
    flex-shrink: 0;
  }

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
</style>
