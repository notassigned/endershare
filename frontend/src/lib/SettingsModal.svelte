<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { GetPeers, RemovePeer, BindPeerWithPhrase, IsMaster } from '../../wailsjs/go/main/App';
  import { showSettings, isLoading, errorMessage } from './stores';

  interface PeerInfo {
    peerId: string;
    isOnline: boolean;
    lastSeen: string;
  }

  let peers: PeerInfo[] = [];
  let isMaster = false;
  let bindPhrase = '';
  let showBindInput = false;
  let showRemoveConfirm = false;
  let peerToRemove: string | null = null;
  let pollInterval: ReturnType<typeof setInterval>;

  onMount(async () => {
    await loadPeers();
    isMaster = await IsMaster();
    pollInterval = setInterval(loadPeers, 5000);
  });

  onDestroy(() => {
    clearInterval(pollInterval);
  });

  async function loadPeers() {
    try {
      peers = await GetPeers();
    } catch (err) {
      errorMessage.set(String(err));
    }
  }

  function confirmRemovePeer(peerID: string) {
    peerToRemove = peerID;
    showRemoveConfirm = true;
  }

  function cancelRemove() {
    showRemoveConfirm = false;
    peerToRemove = null;
  }

  async function handleRemovePeer() {
    if (!peerToRemove) return;

    const peerID = peerToRemove;
    showRemoveConfirm = false;
    peerToRemove = null;

    isLoading.set(true);
    try {
      await RemovePeer(peerID);
      await loadPeers();
    } catch (err) {
      errorMessage.set(String(err));
    } finally {
      isLoading.set(false);
    }
  }

  async function handleBindPeer() {
    if (!bindPhrase.trim()) return;

    isLoading.set(true);
    try {
      await BindPeerWithPhrase(bindPhrase.trim());
      bindPhrase = '';
      showBindInput = false;
      await loadPeers();
    } catch (err) {
      errorMessage.set(String(err));
    } finally {
      isLoading.set(false);
    }
  }

  function close() {
    showSettings.set(false);
  }

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === 'Escape') {
      close();
    } else if (e.key === 'Enter' && showBindInput) {
      handleBindPeer();
    }
  }
</script>

<svelte:window on:keydown={handleKeydown} />

<div class="modal-overlay" on:click={close}>
  <div class="modal" on:click|stopPropagation>
    <div class="modal-header">
      <h2>Settings</h2>
      <button class="close-btn" on:click={close}>✕</button>
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
                <span class="status-dot" class:online={peer.isOnline}></span>
                <span class="peer-id">{peer.peerId}</span>
              </div>
              <div class="peer-meta">
                <span class="last-seen">
                  {peer.isOnline ? 'Online' : peer.lastSeen}
                </span>
                <button
                  class="remove-btn"
                  on:click={() => confirmRemovePeer(peer.peerId)}
                  title="Remove device"
                >
                  ✕
                </button>
              </div>
            </div>
          {/each}
        </div>
      {/if}

      {#if isMaster}
        <div class="add-device-section">
          {#if showBindInput}
            <div class="bind-input-row">
              <input
                type="text"
                class="bind-input"
                bind:value={bindPhrase}
                placeholder="Enter 4-word phrase..."
                autofocus
              />
              <button class="action-btn" on:click={handleBindPeer} disabled={$isLoading}>
                Bind
              </button>
              <button class="action-btn secondary" on:click={() => { showBindInput = false; bindPhrase = ''; }}>
                Cancel
              </button>
            </div>
          {:else}
            <button class="add-device-btn" on:click={() => showBindInput = true}>
              + Add New Device
            </button>
          {/if}
        </div>
      {/if}
    </div>

    <div class="section">
      <h3>Node Info</h3>
      <p class="node-type">
        Node type: <strong>{isMaster ? 'Master' : 'Replica'}</strong>
      </p>
      {#if !isMaster}
        <p class="replica-note">
          Replica nodes sync data but cannot decrypt files without the recovery phrase.
        </p>
      {/if}
    </div>
  </div>
</div>

<!-- Remove device confirmation modal -->
{#if showRemoveConfirm && peerToRemove}
  <div class="confirm-overlay" on:click={cancelRemove} role="dialog" aria-modal="true">
    <div class="confirm-modal" on:click|stopPropagation role="document">
      <h3>Remove Device</h3>
      <p>Are you sure you want to remove device {peerToRemove}?</p>
      <div class="confirm-buttons">
        <button class="cancel-btn" on:click={cancelRemove}>Cancel</button>
        <button class="remove-confirm-btn" on:click={handleRemovePeer}>Remove</button>
      </div>
    </div>
  </div>
{/if}

<style>
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
    border-radius: 12px;
    padding: 1.5rem;
    max-width: 500px;
    width: 90%;
    max-height: 80vh;
    overflow-y: auto;
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

  .section {
    margin-bottom: 2rem;
  }

  .section:last-child {
    margin-bottom: 0;
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
    background: #1a1a1a;
    border-radius: 6px;
  }

  .peer-info {
    display: flex;
    align-items: center;
    gap: 0.75rem;
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
    font-family: monospace;
    font-size: 0.9rem;
  }

  .peer-meta {
    display: flex;
    align-items: center;
    gap: 1rem;
  }

  .last-seen {
    color: #666;
    font-size: 0.85rem;
  }

  .remove-btn {
    background: none;
    border: none;
    color: #666;
    cursor: pointer;
    padding: 0.25rem;
  }

  .remove-btn:hover {
    color: #ff4a4a;
  }

  .add-device-section {
    margin-top: 1rem;
  }

  .bind-input-row {
    display: flex;
    gap: 0.5rem;
  }

  .bind-input {
    flex: 1;
    padding: 0.5rem 0.75rem;
    background: #1a1a1a;
    border: 1px solid #3a3a3a;
    border-radius: 4px;
    color: white;
    font-family: monospace;
  }

  .bind-input:focus {
    outline: none;
    border-color: #4a9eff;
  }

  .action-btn {
    padding: 0.5rem 1rem;
    background: #4a9eff;
    border: none;
    border-radius: 4px;
    color: white;
    cursor: pointer;
  }

  .action-btn:hover:not(:disabled) {
    background: #3a8eef;
  }

  .action-btn:disabled {
    opacity: 0.6;
    cursor: not-allowed;
  }

  .action-btn.secondary {
    background: #3a3a3a;
  }

  .action-btn.secondary:hover {
    background: #4a4a4a;
  }

  .add-device-btn {
    padding: 0.75rem 1rem;
    background: #3a3a3a;
    border: none;
    border-radius: 6px;
    color: white;
    cursor: pointer;
    width: 100%;
  }

  .add-device-btn:hover {
    background: #4a4a4a;
  }

  .node-type {
    color: #ccc;
  }

  .replica-note {
    color: #888;
    font-size: 0.85rem;
    margin-top: 0.5rem;
  }

  .confirm-overlay {
    position: fixed;
    top: 0;
    left: 0;
    right: 0;
    bottom: 0;
    background: rgba(0, 0, 0, 0.8);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 200;
  }

  .confirm-modal {
    background: #2a2a2a;
    border-radius: 12px;
    padding: 1.5rem;
    max-width: 400px;
    width: 90%;
  }

  .confirm-modal h3 {
    margin: 0 0 0.75rem 0;
    font-size: 1.1rem;
  }

  .confirm-modal p {
    color: #ccc;
    margin-bottom: 1.5rem;
    word-break: break-all;
  }

  .confirm-buttons {
    display: flex;
    gap: 0.75rem;
    justify-content: flex-end;
  }

  .cancel-btn {
    padding: 0.625rem 1.25rem;
    background: #3a3a3a;
    color: white;
    border: none;
    border-radius: 6px;
    cursor: pointer;
    font-size: 0.9rem;
  }

  .cancel-btn:hover {
    background: #4a4a4a;
  }

  .remove-confirm-btn {
    padding: 0.625rem 1.25rem;
    background: #dc2626;
    color: white;
    border: none;
    border-radius: 6px;
    cursor: pointer;
    font-size: 0.9rem;
  }

  .remove-confirm-btn:hover {
    background: #b91c1c;
  }
</style>
