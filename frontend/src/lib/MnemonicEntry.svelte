<script lang="ts">
  import { UnlockWithMnemonic } from '../../wailsjs/go/main/App';
  import { appState, isLoading, errorMessage } from './stores';

  let mnemonic = '';

  async function unlock() {
    if (!mnemonic.trim()) {
      errorMessage.set('Please enter your recovery phrase');
      return;
    }

    isLoading.set(true);
    errorMessage.set('');

    try {
      await UnlockWithMnemonic(mnemonic.trim());
      appState.set('unlocked');
    } catch (err) {
      errorMessage.set(String(err));
    } finally {
      isLoading.set(false);
    }
  }

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === 'Enter') {
      unlock();
    }
  }
</script>

<div class="mnemonic-screen">
  <h1>Unlock Vault</h1>

  <p class="instruction">
    Enter your 24-word recovery phrase to decrypt your files.
  </p>

  <textarea
    class="mnemonic-input"
    bind:value={mnemonic}
    on:keydown={handleKeydown}
    placeholder="Enter your recovery phrase..."
    rows="4"
    disabled={$isLoading}
  ></textarea>

  <button class="unlock-btn" on:click={unlock} disabled={$isLoading || !mnemonic.trim()}>
    {$isLoading ? 'Unlocking...' : 'Unlock'}
  </button>

  {#if $errorMessage}
    <p class="error">{$errorMessage}</p>
  {/if}

  <p class="note">
    Only master nodes with the recovery phrase can view files.
    Replica nodes cannot decrypt file contents.
  </p>
</div>

<style>
  .mnemonic-screen {
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

  .mnemonic-input {
    width: 100%;
    max-width: 400px;
    padding: 1rem;
    background: #2a2a2a;
    border: 1px solid #3a3a3a;
    border-radius: 0;
    color: white;
    font-size: 1rem;
    resize: none;
    margin-bottom: 1.5rem;
  }

  .mnemonic-input:focus {
    outline: none;
    border-color: #4a9eff;
  }

  .mnemonic-input::placeholder {
    color: #666;
  }

  .unlock-btn {
    padding: 0.875rem 2.5rem;
    background: #4a9eff;
    color: white;
    border: none;
    border-radius: 0;
    cursor: pointer;
    font-size: 1rem;
    font-weight: 600;
  }

  .unlock-btn:hover:not(:disabled) {
    background: #3a8eef;
  }

  .unlock-btn:disabled {
    opacity: 0.6;
    cursor: not-allowed;
  }

  .error {
    color: #ff4a4a;
    margin-top: 1rem;
  }

  .note {
    color: #666;
    font-size: 0.85rem;
    margin-top: 2rem;
    max-width: 350px;
  }
</style>
