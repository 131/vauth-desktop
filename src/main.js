import { flushSync, mount } from 'svelte';
import App from './App.svelte';
import 'vauth-ui/style.css';
import './desktop.css';
import controller from '../ui/app.js';

// Populate the stable shared DOM before the native controller attaches once.
flushSync(() => mount(App, { target: document.body }));
// The controller is bundled as text, but executed only after the Svelte mount.
const script = document.createElement('script');
script.textContent = controller;
document.body.append(script);
