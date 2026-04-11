import DefaultTheme from 'vitepress/theme'
import type { Theme } from 'vitepress'
import Argon2Generator from './components/Argon2Generator.vue'

export default {
  extends: DefaultTheme,
  enhanceApp({ app }) {
    app.component('Argon2Generator', Argon2Generator)
  },
} satisfies Theme
