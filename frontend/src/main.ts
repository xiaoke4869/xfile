import { createApp } from 'vue'
import { System } from '@wailsio/runtime'
import App from './App.vue'
import { native } from './api'
import './style.css'
// The runtime environment may arrive after the page starts in a native WebView.
if(native&&(System.IsMac()||/Mac/i.test(navigator.platform)))document.documentElement.classList.add('native-mac')
createApp(App).mount('#app')
