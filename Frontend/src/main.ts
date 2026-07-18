import { mount } from 'svelte'
import './app.css'
import App from './App.svelte'
import { applyTheme, theme } from './lib/stores/theme'

// Apply the last-known theme immediately, before the app renders, so
// there's no flash of the wrong theme on load. App.svelte reconciles
// this with the server-saved preference once the profile loads.
let initial: "light" | "dark" | "system" = "system"
theme.subscribe((t) => (initial = t))()
applyTheme(initial)

const app = mount(App, {
  target: document.getElementById('app')!,
})

export default app
