import { mount } from 'svelte'
import './app.css'
import App from './App.svelte'
import { applyTheme, theme } from './lib/stores/theme'

// Apply the last-known theme immediately on startup
let initial: "light" | "dark" = "light"
theme.subscribe((t) => (initial = t))()
applyTheme(initial)

const app = mount(App, {
  target: document.getElementById('app')!,
})

export default app
