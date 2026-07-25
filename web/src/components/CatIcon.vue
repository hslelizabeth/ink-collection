<script setup>
// 品类图标：pen / ink / inkstone / inkstick / generic / home / image
const props = defineProps({
  name: { type: String, default: 'generic' }
})

const paths = {
  pen: [
    { tag: 'path', d: 'M12 2.5c3.6 0 5.8 2.1 5.8 5.3 0 4.2-2.6 8.3-5.8 11.7-3.2-3.4-5.8-7.5-5.8-11.7C6.2 4.6 8.4 2.5 12 2.5z' },
    { tag: 'path', d: 'M12 19.5v-7.2' },
    { tag: 'circle', cx: 12, cy: 10.3, r: 1.3 }
  ],
  ink: [
    { tag: 'path', d: 'M12 3c3 4.5 6 8 6 11a6 6 0 01-12 0c0-3 3-6.5 6-11z' }
  ],
  inkstone: [
    { tag: 'rect', x: 3, y: 5.5, width: 18, height: 13, rx: 2 },
    { tag: 'rect', x: 6.5, y: 8.5, width: 11, height: 7, rx: 1 },
    { tag: 'path', d: 'M8.8 12q1.6-1.4 3.2 0t3.2 0' }
  ],
  inkstick: [
    { tag: 'rect', x: 9, y: 4, width: 6, height: 16, rx: 1.5 },
    { tag: 'path', d: 'M9 8h6M12 11v5' }
  ],
  // 新增品类通用图标：抽屉藏匣
  generic: [
    { tag: 'rect', x: 4, y: 5, width: 16, height: 14, rx: 1.5 },
    { tag: 'path', d: 'M4 12h16' },
    { tag: 'path', d: 'M10 8.5h4M10 15.5h4' }
  ],
  home: [
    { tag: 'path', d: 'M3 11l9-8 9 8' },
    { tag: 'path', d: 'M5 10v10h14V10' }
  ],
  image: [
    { tag: 'rect', x: 3, y: 5, width: 18, height: 14, rx: 2 },
    { tag: 'circle', cx: 9, cy: 10, r: 1.6 },
    { tag: 'path', d: 'M5 17l4-4 3 3 3-3 4 4' }
  ]
}
function attrsOf(p) {
  const { tag, ...rest } = p
  return rest
}
</script>

<template>
  <svg viewBox="0 0 24 24" fill="none" stroke-width="1.4" stroke="currentColor">
    <template v-for="(p, i) in (paths[props.name] || paths.generic)" :key="i">
      <rect v-if="p.tag === 'rect'" v-bind="attrsOf(p)" />
      <circle v-else-if="p.tag === 'circle'" v-bind="attrsOf(p)" />
      <path v-else :d="p.d" />
    </template>
  </svg>
</template>
