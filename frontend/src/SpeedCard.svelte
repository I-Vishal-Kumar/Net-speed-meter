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
  .bar {
    width: 86px;
    height: 36px;
    display: flex;
    flex-direction: column;
    justify-content: center;
    align-items: flex-end;
    gap: 0;
    background: transparent;
    cursor: default;
  }

  .row {
    display: flex;
    align-items: center;
    gap: 1px;
    height: 17px;
    width: 100%;
    line-height: 17px;
  }

  .arrow {
    font-size: 7px;
    width: 10px;
    text-align: center;
    flex-shrink: 0;
  }

  .arrow.dl { color: #4cc2ff; }
  .arrow.ul { color: #ff6b6b; }

  .num {
    font-size: 11.5px;
    font-weight: 600;
    color: #ffffff;
    font-variant-numeric: tabular-nums;
    min-width: 28px;
    text-align: right;
    letter-spacing: -0.3px;
    padding-right: 1px;
  }

  .unit {
    font-size: 9px;
    font-weight: 400;
    color: rgba(255, 255, 255, 0.55);
    letter-spacing: 0.2px;
    width: 26px;
  }
</style>
