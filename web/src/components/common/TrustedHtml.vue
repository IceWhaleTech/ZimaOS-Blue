<script lang="ts">
import { defineComponent, h } from 'vue'

export default defineComponent({
  name: 'TrustedHtml',
  inheritAttrs: false,
  props: {
    html: {
      type: String,
      required: true,
    },
    tag: {
      type: String,
      default: 'div',
    },
  },
  setup(props, { attrs }) {
    return () => h(props.tag, { ...attrs, innerHTML: props.html })
  },
})
</script>
