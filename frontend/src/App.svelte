<script lang="ts">
  import { onMount } from 'svelte';
  import { GetAppState } from '../wailsjs/go/main/App';
  import { appState } from './lib/stores';
  import SetupScreen from './lib/SetupScreen.svelte';
  import BindingScreen from './lib/BindingScreen.svelte';
  import NodeDashboard from './lib/NodeDashboard.svelte';
  import FileBrowser from './lib/FileBrowser.svelte';

  onMount(async () => {
    const state = await GetAppState();
    appState.set(state);
  });
</script>

<main>
  {#if $appState === 'fresh'}
    <SetupScreen />
  {:else if $appState === 'binding'}
    <BindingScreen />
  {:else if $appState === 'locked'}
    <NodeDashboard fullPage={true} />
  {:else if $appState === 'unlocked'}
    <FileBrowser />
  {:else}
    <div class="loading">Loading...</div>
  {/if}
</main>

<style>
  main {
    height: 100vh;
    width: 100vw;
    overflow: hidden;
  }

  .loading {
    display: flex;
    align-items: center;
    justify-content: center;
    height: 100%;
    color: #888;
  }
</style>
