<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { GetPeers, GetStorageStats, GetNodeID, UnlockWithMnemonic } from '../../wailsjs/go/main/App';
  import { appState, showDashboard, isLoading, errorMessage } from './stores';
  import computerIcon from '../assets/images/computer.png';

  // When true, renders as full-page (locked state). When false, renders as modal content.
  export let fullPage = false;

  interface PeerInfo {
    peerId: string;
    isOnline: boolean;
    lastSeen: string;
  }

  interface StorageStats {
    entryCount: number;
    totalSize: number;
  }

  let peers: PeerInfo[] = [];
  let stats: StorageStats = { entryCount: 0, totalSize: 0 };
  let nodeId = '';
  let showUnlockInput = false;
  let mnemonic = '';
  let pollInterval: ReturnType<typeof setInterval>;

  onMount(() => {
    loadData();
    pollInterval = setInterval(loadData, 5000);
  });

  onDestroy(() => {
    clearInterval(pollInterval);
  });

  async function loadData() {
    try {
      const [p, s, id] = await Promise.all([GetPeers(), GetStorageStats(), GetNodeID()]);
      peers = p;
      stats = s;
      nodeId = id;
    } catch (err) {
      errorMessage.set(String(err));
    }
  }

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
      showDashboard.set(false);
    } catch (err) {
      errorMessage.set(String(err));
    } finally {
      isLoading.set(false);
    }
  }

  function handleUnlockKeydown(e: KeyboardEvent) {
    if (e.key === 'Enter') unlock();
    if (e.key === 'Escape') {
      showUnlockInput = false;
      mnemonic = '';
      errorMessage.set('');
    }
  }

  function close() {
    showDashboard.set(false);
  }

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === 'Escape' && !fullPage) close();
  }

  function formatSize(bytes: number): string {
    if (bytes === 0) return '0 B';
    const units = ['B', 'KB', 'MB', 'GB', 'TB'];
    const i = Math.floor(Math.log(bytes) / Math.log(1024));
    return (bytes / Math.pow(1024, i)).toFixed(1) + ' ' + units[i];
  }
</script>

<svelte:window on:keydown={handleKeydown} />

{#if fullPage}
  <!-- Full-page layout for locked state -->
  <div class="dashboard-page">
    <div class="top-bar">
      <h2>Node Dashboard</h2>
      {#if showUnlockInput}
        <div class="unlock-row">
          <input
            type="password"
            class="mnemonic-input"
            bind:value={mnemonic}
            on:keydown={handleUnlockKeydown}
            placeholder="Enter 24-word recovery phrase..."
            disabled={$isLoading}
            autofocus
          />
          <button class="unlock-btn" on:click={unlock} disabled={$isLoading || !mnemonic.trim()}>
            {$isLoading ? 'Unlocking...' : 'Unlock'}
          </button>
          <button class="cancel-btn" on:click={() => { showUnlockInput = false; mnemonic = ''; errorMessage.set(''); }}>
            Cancel
          </button>
        </div>
      {:else}
        <button class="unlock-btn" on:click={() => showUnlockInput = true}>
          Unlock Vault
        </button>
      {/if}
    </div>

    <div class="dashboard-content">
      {#if nodeId}
        <div class="node-id-section">
          <span class="node-id-label">Node ID</span>
          <span class="node-id-value">{nodeId}</span>
        </div>
      {/if}

      <div class="stats-row">
        <div class="stat-card">
          <span class="stat-value">{stats.entryCount}</span>
          <span class="stat-label">Entries</span>
        </div>
        <div class="stat-card">
          <span class="stat-value">{formatSize(stats.totalSize)}</span>
          <span class="stat-label">Total Size</span>
        </div>
        <div class="stat-card">
          <span class="stat-value">{peers.filter(p => p.isOnline).length}/{peers.length}</span>
          <span class="stat-label">Peers Online</span>
        </div>
      </div>

      <div class="section">
        <h3>Connected Devices</h3>
        {#if peers.length === 0}
          <p class="empty">No connected devices</p>
        {:else}
          <div class="peer-list">
            {#each peers as peer}
              <div class="peer-item">
                <div class="peer-info">
                  <img class="peer-icon" src={computerIcon} alt="device" />
                  <span class="status-dot" class:online={peer.isOnline}></span>
                  <span class="peer-id">{peer.peerId}</span>
                </div>
                <span class="last-seen">
                  {peer.isOnline ? 'Online' : peer.lastSeen}
                </span>
              </div>
            {/each}
          </div>
        {/if}
      </div>
    </div>

    {#if $errorMessage}
      <div class="error-bar">
        {$errorMessage}
        <button on:click={() => errorMessage.set('')}>✕</button>
      </div>
    {/if}
  </div>
{:else}
  <!-- Modal layout for use from FileBrowser -->
  <div class="modal-overlay" on:click={close}>
    <div class="modal" on:click|stopPropagation>
      <div class="modal-header">
        <h2>Node Dashboard</h2>
        <button class="close-btn" on:click={close}>✕</button>
      </div>

      {#if nodeId}
        <div class="node-id-section">
          <span class="node-id-label">Node ID</span>
          <span class="node-id-value">{nodeId}</span>
        </div>
      {/if}

      <div class="stats-row">
        <div class="stat-card">
          <span class="stat-value">{stats.entryCount}</span>
          <span class="stat-label">Entries</span>
        </div>
        <div class="stat-card">
          <span class="stat-value">{formatSize(stats.totalSize)}</span>
          <span class="stat-label">Total Size</span>
        </div>
        <div class="stat-card">
          <span class="stat-value">{peers.filter(p => p.isOnline).length}/{peers.length}</span>
          <span class="stat-label">Peers Online</span>
        </div>
      </div>

      <div class="section">
        <h3>Connected Devices</h3>
        {#if peers.length === 0}
          <p class="empty">No connected devices</p>
        {:else}
          <div class="peer-list">
            {#each peers as peer}
              <div class="peer-item">
                <div class="peer-info">
                  <img class="peer-icon" src={computerIcon} alt="device" />
                  <span class="status-dot" class:online={peer.isOnline}></span>
                  <span class="peer-id">{peer.peerId}</span>
                </div>
                <span class="last-seen">
                  {peer.isOnline ? 'Online' : peer.lastSeen}
                </span>
              </div>
            {/each}
          </div>
        {/if}
      </div>
    </div>
  </div>
{/if}

<style>
  /* Full-page layout */
  .dashboard-page {
    display: flex;
    flex-direction: column;
    height: 100%;
  }

  .top-bar {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 0.75rem 1rem;
    background: #2a2a2a;
    border-bottom: 1px solid #3a3a3a;
    gap: 1rem;
  }

  .top-bar h2 {
    margin: 0;
    font-size: 1.1rem;
    white-space: nowrap;
  }

  .unlock-row {
    display: flex;
    gap: 0.5rem;
    align-items: center;
    flex: 1;
    justify-content: flex-end;
  }

  .mnemonic-input {
    padding: 0.5rem 0.75rem;
    background: #1a1a1a;
    border: 1px solid #3a3a3a;
    border-radius: 0;
    color: white;
    font-size: 0.85rem;
    width: 300px;
  }

  .mnemonic-input:focus {
    outline: none;
    border-color: #4a9eff;
  }

  .unlock-btn {
    padding: 0.5rem 1rem;
    background: #4a9eff;
    color: white;
    border: none;
    border-radius: 0;
    cursor: pointer;
    font-weight: 600;
    white-space: nowrap;
  }

  .unlock-btn:hover:not(:disabled) {
    background: #3a8eef;
  }

  .unlock-btn:disabled {
    opacity: 0.6;
    cursor: not-allowed;
  }

  .cancel-btn {
    padding: 0.5rem 1rem;
    background: #3a3a3a;
    color: white;
    border: none;
    border-radius: 0;
    cursor: pointer;
    white-space: nowrap;
  }

  .cancel-btn:hover {
    background: #4a4a4a;
  }

  /* Dashboard content */
  .dashboard-content {
    flex: 1;
    overflow-y: auto;
    padding: 1.5rem;
  }

  .node-id-section {
    display: flex;
    align-items: center;
    gap: 0.75rem;
    margin-bottom: 1.5rem;
    padding: 0.75rem 1rem;
    background: #2a2a2a;
    border-radius: 0;
  }

  .modal .node-id-section {
    background: #1a1a1a;
  }

  .node-id-label {
    font-size: 0.8rem;
    color: #888;
    text-transform: uppercase;
    letter-spacing: 0.05em;
  }

  .node-id-value {
    font-size: 0.9rem;
    color: #ccc;
  }

  .stats-row {
    display: flex;
    gap: 1rem;
    margin-bottom: 2rem;
  }

  .stat-card {
    flex: 1;
    background: #2a2a2a;
    border-radius: 0;
    padding: 1.25rem;
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 0.5rem;
  }

  .stat-value {
    font-size: 1.5rem;
    font-weight: 700;
    color: #4a9eff;
  }

  .stat-label {
    font-size: 0.8rem;
    color: #888;
    text-transform: uppercase;
    letter-spacing: 0.05em;
  }

  .section {
    margin-bottom: 1.5rem;
  }

  h3 {
    font-size: 0.9rem;
    color: #888;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    margin-bottom: 1rem;
  }

  .empty {
    color: #666;
    font-style: italic;
  }

  .peer-list {
    display: flex;
    flex-direction: column;
    gap: 0.5rem;
  }

  .peer-item {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 0.75rem 1rem;
    background: #2a2a2a;
    border-radius: 0;
  }

  /* In modal mode, peer items need darker bg */
  .modal .peer-item {
    background: #1a1a1a;
  }

  .peer-info {
    display: flex;
    align-items: center;
    gap: 0.75rem;
  }

  .peer-icon {
    width: 20px;
    height: 20px;
    image-rendering: pixelated;
  }

  .status-dot {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    background: #666;
  }

  .status-dot.online {
    background: #4ade80;
  }

  .peer-id {
    font-size: 0.9rem;
  }

  .last-seen {
    color: #666;
    font-size: 0.85rem;
  }

  .error-bar {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 0.75rem 1rem;
    background: #3a1a1a;
    color: #ff6a6a;
  }

  .error-bar button {
    background: none;
    border: none;
    color: #ff6a6a;
    cursor: pointer;
    padding: 0.25rem;
  }

  /* Modal layout */
  .modal-overlay {
    position: fixed;
    top: 0;
    left: 0;
    right: 0;
    bottom: 0;
    background: rgba(0, 0, 0, 0.7);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 100;
  }

  .modal {
    background: #2a2a2a;
    border-radius: 0;
    padding: 1.5rem;
    max-width: 500px;
    width: 90%;
    max-height: 80vh;
    overflow-y: auto;
  }

  .modal .stats-row {
    margin-bottom: 1.5rem;
  }

  .modal .stat-card {
    background: #1a1a1a;
  }

  .modal-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 1.5rem;
  }

  .modal-header h2 {
    margin: 0;
  }

  .close-btn {
    background: none;
    border: none;
    color: #888;
    font-size: 1.25rem;
    cursor: pointer;
    padding: 0.25rem;
  }

  .close-btn:hover {
    color: white;
  }
</style>
