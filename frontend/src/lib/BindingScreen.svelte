<script lang="ts">
  import { GetSyncPhrase, CancelBinding } from '../../wailsjs/go/main/App';
  import { EventsOn } from '../../wailsjs/runtime/runtime';
  import { appState, isLoading } from './stores';
  import { onMount, onDestroy } from 'svelte';

  let syncPhrase = '';
  let unsubscribe: () => void;

  onMount(async () => {
    syncPhrase = await GetSyncPhrase();

    // Listen for binding completion event
    unsubscribe = EventsOn('binding-complete', () => {
      appState.set('locked');
    });
  });

  onDestroy(() => {
    if (unsubscribe) unsubscribe();
  });

  async function cancel() {
    isLoading.set(true);
    await CancelBinding();
    appState.set('fresh');
    isLoading.set(false);
  }
</script>

<div class="binding-screen">
  <h1>Waiting for Authorization</h1>

  <p class="instruction">
    Enter this phrase on your master device to authorize this replica:
  </p>

  <div class="phrase-display">
    {syncPhrase || 'Loading...'}
  </div>

  <div class="spinner"></div>

  <p class="waiting-text">Waiting for master node to connect...</p>

  <button class="cancel-btn" on:click={cancel} disabled={$isLoading}>
    Cancel
  </button>
</div>

<style>
  .binding-screen {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    height: 100%;
    padding: 2rem;
    text-align: center;
  }

  h1 {
    font-size: 1.75rem;
    margin-bottom: 1rem;
  }

  .instruction {
    color: #888;
    margin-bottom: 2rem;
    max-width: 400px;
  }

  .phrase-display {
    font-size: 1.5rem;
    font-weight: 600;
    padding: 1.5rem 2rem;
    background: #2a2a2a;
    border-radius: 8px;
    letter-spacing: 0.1em;
    margin-bottom: 2rem;
    font-family: monospace;
  }

  .spinner {
    width: 32px;
    height: 32px;
    border: 3px solid #333;
    border-top-color: #4a9eff;
    border-radius: 50%;
    animation: spin 1s linear infinite;
    margin-bottom: 1rem;
  }

  @keyframes spin {
    to { transform: rotate(360deg); }
  }

  .waiting-text {
    color: #666;
    margin-bottom: 2rem;
  }

  .cancel-btn {
    padding: 0.75rem 2rem;
    background: #3a3a3a;
    color: white;
    border: none;
    border-radius: 6px;
    cursor: pointer;
    font-size: 1rem;
  }

  .cancel-btn:hover:not(:disabled) {
    background: #4a4a4a;
  }

  .cancel-btn:disabled {
    opacity: 0.6;
    cursor: not-allowed;
  }
</style>
