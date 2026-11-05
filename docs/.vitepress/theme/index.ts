import DefaultTheme from 'vitepress/theme'
import { useData } from 'vitepress'
import { defineComponent, h } from 'vue'

import './custom.css'
import HomeLayout from './components/HomeLayout.vue'

export default {
  extends: DefaultTheme,
  Layout: defineComponent({
    name: 'WeaverLayout',
    setup() {
      const { frontmatter } = useData()
      return () => (frontmatter.value.layout === 'home' ? h(HomeLayout) : h(DefaultTheme.Layout))
    },
  }),
}
