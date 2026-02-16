<script lang="ts">
  import { CreateNewVault, StartReplicaBinding } from '../../wailsjs/go/main/App';
  import { appState, displayMnemonic, isLoading, errorMessage } from './stores';

  async function createNewVault() {
    isLoading.set(true);
    errorMessage.set('');
    try {
      const mnemonic = await CreateNewVault();
      displayMnemonic.set(mnemonic);
      appState.set('unlocked');
    } catch (err) {
      errorMessage.set(String(err));
    } finally {
      isLoading.set(false);
    }
  }

  async function joinExistingVault() {
    isLoading.set(true);
    errorMessage.set('');
    try {
      await StartReplicaBinding();
      appState.set('binding');
    } catch (err) {
      errorMessage.set(String(err));
    } finally {
      isLoading.set(false);
    }
  }
</script>

<div class="setup-screen">
  <h1>Endershare</h1>
  <p class="subtitle">Encrypted P2P File Storage</p>

  <div class="setup-options">
    <button class="setup-btn primary" on:click={createNewVault} disabled={$isLoading}>
      Create New Vault
      <span class="btn-desc">Generate a new encryption key</span>
    </button>

    <button class="setup-btn secondary" on:click={joinExistingVault} disabled={$isLoading}>
      Join Existing Vault
      <span class="btn-desc">Connect to another device</span>
    </button>
  </div>

  {#if $errorMessage}
    <p class="error">{$errorMessage}</p>
  {/if}
</div>

<style>
  .setup-screen {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    height: 100%;
    padding: 2rem;
  }

  h1 {
    font-size: 2.5rem;
    margin-bottom: 0.5rem;
  }

  .subtitle {
    color: #888;
    margin-bottom: 3rem;
  }

  .setup-options {
    display: flex;
    flex-direction: column;
    gap: 1rem;
    width: 100%;
    max-width: 300px;
  }

  .setup-btn {
    display: flex;
    flex-direction: column;
    padding: 1.25rem;
    border: none;
    border-radius: 0;
    cursor: pointer;
    font-size: 1rem;
    font-weight: 600;
  }

  .setup-btn:hover:not(:disabled) {
    filter: brightness(1.15);
  }

  .setup-btn:disabled {
    opacity: 0.6;
    cursor: not-allowed;
  }

  .setup-btn.primary {
    background: #4a9eff;
    color: white;
  }

  .setup-btn.secondary {
    background: #3a3a3a;
    color: white;
  }

  .btn-desc {
    font-size: 0.8rem;
    font-weight: normal;
    opacity: 0.8;
    margin-top: 0.25rem;
  }

  .error {
    color: #ff4a4a;
    margin-top: 1rem;
  }
</style>
