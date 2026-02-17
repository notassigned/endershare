<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { EventsOn } from '../../wailsjs/runtime/runtime';
  import {
    ListFolder,
    CreateFolder,
    AddFile,
    ExportFile,
    DeleteFile,
    DeleteFolder,
    GetFolderPath,
    IsMaster
  } from '../../wailsjs/go/main/App';
  import { currentFolderID, showSettings, showDashboard, displayMnemonic, isLoading, errorMessage } from './stores';
  import SettingsModal from './SettingsModal.svelte';
  import NodeDashboard from './NodeDashboard.svelte';
  import folderIcon from '../assets/images/directory.png';
  import fileIcon from '../assets/images/file.png';

  interface FolderItem {
    type: string;
    name: string;
    folderId: number;
    size: number;
    modifiedAt: string;
  }

  interface PathSegment {
    name: string;
    folderId: number;
  }

  let items: FolderItem[] = [];
  let pathSegments: PathSegment[] = [];
  let isMaster = false;
  let newFolderName = '';
  let showNewFolderInput = false;
  let showMnemonicModal = false;
  let showDeleteConfirm = false;
  let itemToDelete: FolderItem | null = null;
  let unsubscribeDataUpdated: (() => void) | null = null;

  $: loadFolder($currentFolderID);

  onMount(async () => {
    isMaster = await IsMaster();

    // Check if we need to show the mnemonic modal
    if ($displayMnemonic) {
      showMnemonicModal = true;
    }

    // Listen for data updates from other devices
    unsubscribeDataUpdated = EventsOn('data-updated', () => {
      loadFolder($currentFolderID);
    });
  });

  onDestroy(() => {
    if (unsubscribeDataUpdated) {
      unsubscribeDataUpdated();
    }
  });

  async function loadFolder(folderID: number) {
    try {
      items = await ListFolder(folderID);
      pathSegments = await GetFolderPath(folderID);
    } catch (err) {
      errorMessage.set(String(err));
    }
  }

  function navigateToFolder(folderID: number) {
    currentFolderID.set(folderID);
  }

  function navigateUp() {
    if (pathSegments.length > 1) {
      const parentSegment = pathSegments[pathSegments.length - 2];
      currentFolderID.set(parentSegment.folderId);
    }
  }

  function handleItemClick(item: FolderItem) {
    if (item.type === 'folder') {
      currentFolderID.set(item.folderId);
    }
  }

  async function handleAddFile() {
    isLoading.set(true);
    try {
      await AddFile($currentFolderID);
      await loadFolder($currentFolderID);
    } catch (err) {
      errorMessage.set(String(err));
    } finally {
      isLoading.set(false);
    }
  }

  async function handleCreateFolder() {
    if (!newFolderName.trim()) return;

    isLoading.set(true);
    try {
      await CreateFolder(newFolderName.trim(), $currentFolderID);
      newFolderName = '';
      showNewFolderInput = false;
      await loadFolder($currentFolderID);
    } catch (err) {
      errorMessage.set(String(err));
    } finally {
      isLoading.set(false);
    }
  }

  async function handleExport(item: FolderItem) {
    isLoading.set(true);
    try {
      await ExportFile(item.name, $currentFolderID);
    } catch (err) {
      errorMessage.set(String(err));
    } finally {
      isLoading.set(false);
    }
  }

  function confirmDelete(item: FolderItem) {
    itemToDelete = item;
    showDeleteConfirm = true;
  }

  function cancelDelete() {
    showDeleteConfirm = false;
    itemToDelete = null;
  }

  async function handleDelete() {
    if (!itemToDelete) return;

    const item = itemToDelete;
    showDeleteConfirm = false;
    itemToDelete = null;

    isLoading.set(true);
    try {
      if (item.type === 'folder') {
        await DeleteFolder(item.folderId);
      } else {
        await DeleteFile(item.name, $currentFolderID);
      }
      await loadFolder($currentFolderID);
    } catch (err) {
      errorMessage.set(String(err));
    } finally {
      isLoading.set(false);
    }
  }

  function formatSize(bytes: number): string {
    if (bytes === 0) return '';
    const units = ['B', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(1024));
    return (bytes / Math.pow(1024, i)).toFixed(1) + ' ' + units[i];
  }

  function formatDate(isoString: string): string {
    if (!isoString) return '';
    const date = new Date(isoString);
    return date.toLocaleDateString();
  }

  function closeMnemonicModal() {
    showMnemonicModal = false;
    displayMnemonic.set('');
  }

  function handleFolderKeydown(e: KeyboardEvent) {
    if (e.key === 'Enter') {
      handleCreateFolder();
    } else if (e.key === 'Escape') {
      showNewFolderInput = false;
      newFolderName = '';
    }
  }
</script>

<div class="file-browser">
  <!-- Top bar -->
  <div class="top-bar">
    <div class="path-section">
      <button class="nav-btn" on:click={navigateUp} disabled={$currentFolderID === 0}>
        <span class="icon">↑</span>
      </button>

      <div class="breadcrumb">
        {#each pathSegments as segment, i}
          {#if i > 0}<span class="separator">/</span>{/if}
          <button
            class="path-segment"
            class:current={i === pathSegments.length - 1}
            on:click={() => navigateToFolder(segment.folderId)}
          >
            {segment.folderId === 0 ? 'Home' : segment.name}
          </button>
        {/each}
      </div>
    </div>

    <div class="actions">
      {#if showNewFolderInput}
        <input
          type="text"
          class="folder-input"
          bind:value={newFolderName}
          on:keydown={handleFolderKeydown}
          placeholder="Folder name..."
          autofocus
        />
        <button class="action-btn" on:click={handleCreateFolder}>Create</button>
        <button class="action-btn" on:click={() => { showNewFolderInput = false; newFolderName = ''; }}>Cancel</button>
      {:else}
        <button class="action-btn" on:click={() => showNewFolderInput = true}>
          New Folder
        </button>
        <button class="action-btn" on:click={handleAddFile}>
          <span class="icon">+</span> Add File
        </button>
      {/if}
      <button class="action-btn" on:click={() => showDashboard.set(true)} title="Node Dashboard">
        Dashboard
      </button>
      <button class="action-btn settings" on:click={() => showSettings.set(true)}>
        Settings
      </button>
    </div>
  </div>

  <!-- File list -->
  <div class="file-list">
    {#if items.length === 0}
      <div class="empty-state">
        <p>This folder is empty</p>
        <p class="hint">Add files or create folders to get started</p>
      </div>
    {:else}
      {#each items as item}
        <div
          class="file-item"
          class:folder={item.type === 'folder'}
          on:click={() => handleItemClick(item)}
        >
          <img class="item-icon" src={item.type === 'folder' ? folderIcon : fileIcon} alt={item.type} />
          <span class="item-name">{item.name}</span>
          <span class="item-size">{formatSize(item.size)}</span>
          <span class="item-date">{formatDate(item.modifiedAt)}</span>
          <div class="item-actions">
            {#if item.type === 'file'}
              <button class="item-btn" on:click|stopPropagation={() => handleExport(item)} title="Export">
                ↓
              </button>
            {/if}
            <button class="item-btn delete" on:click|stopPropagation={() => confirmDelete(item)} title="Delete">
              ✕
            </button>
          </div>
        </div>
      {/each}
    {/if}
  </div>

  {#if $errorMessage}
    <div class="error-bar">
      {$errorMessage}
      <button on:click={() => errorMessage.set('')}>✕</button>
    </div>
  {/if}
</div>

{#if $showSettings}
  <SettingsModal />
{/if}

{#if $showDashboard}
  <NodeDashboard />
{/if}

<!-- Delete confirmation modal -->
{#if showDeleteConfirm && itemToDelete}
  <div class="modal-overlay" on:click={cancelDelete} role="dialog" aria-modal="true">
    <div class="modal confirm-modal" on:click|stopPropagation role="document">
      <h2>Delete {itemToDelete.type === 'folder' ? 'Folder' : 'File'}</h2>
      <p>Are you sure you want to delete "{itemToDelete.name}"?</p>
      <div class="modal-buttons">
        <button class="cancel-btn" on:click={cancelDelete}>Cancel</button>
        <button class="delete-btn" on:click={handleDelete}>Delete</button>
      </div>
    </div>
  </div>
{/if}

<!-- Mnemonic display modal -->
{#if showMnemonicModal}
  <div class="modal-overlay" on:click={closeMnemonicModal}>
    <div class="modal" on:click|stopPropagation>
      <h2>Save Your Recovery Phrase</h2>
      <p class="warning">
        Write down these words and keep them safe. You will need them to recover your vault.
      </p>
      <div class="mnemonic-display">
        {$displayMnemonic}
      </div>
      <p class="warning">
        Anyone with this phrase can access your files. Never share it.
      </p>
      <button class="primary-btn" on:click={closeMnemonicModal}>
        I've Saved My Phrase
      </button>
    </div>
  </div>
{/if}

<style>
  .file-browser {
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
  }

  .path-section {
    display: flex;
    align-items: center;
    gap: 0.5rem;
  }

  .nav-btn {
    padding: 0.5rem 0.75rem;
    background: #3a3a3a;
    border: none;
    border-radius: 0;
    color: white;
    cursor: pointer;
  }

  .nav-btn:hover:not(:disabled) {
    background: #4a4a4a;
  }

  .nav-btn:disabled {
    opacity: 0.4;
    cursor: not-allowed;
  }

  .breadcrumb {
    display: flex;
    align-items: center;
    gap: 0.25rem;
  }

  .separator {
    color: #666;
    margin: 0 0.25rem;
  }

  .path-segment {
    background: none;
    border: none;
    color: #888;
    cursor: pointer;
    padding: 0.25rem 0.5rem;
    border-radius: 0;
  }

  .path-segment:hover {
    color: white;
    background: #3a3a3a;
  }

  .path-segment.current {
    color: white;
  }

  .actions {
    display: flex;
    gap: 0.5rem;
    align-items: center;
  }

  .action-btn {
    padding: 0.5rem 1rem;
    background: #3a3a3a;
    border: none;
    border-radius: 0;
    color: white;
    cursor: pointer;
    display: flex;
    align-items: center;
    gap: 0.5rem;
  }

  .action-btn:hover {
    background: #4a4a4a;
  }

  .folder-input {
    padding: 0.5rem;
    background: #1a1a1a;
    border: 1px solid #4a4a4a;
    border-radius: 0;
    color: white;
    width: 150px;
  }

  .file-list {
    flex: 1;
    overflow-y: auto;
    padding: 0.5rem;
  }

  .empty-state {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    height: 100%;
    color: #666;
  }

  .hint {
    font-size: 0.85rem;
    color: #555;
  }

  .file-item {
    display: flex;
    align-items: center;
    padding: 0.75rem 1rem;
    border-radius: 0;
    cursor: pointer;
    gap: 0.75rem;
    user-select: none;
  }

  .file-item:hover {
    background: #2a2a2a;
  }

  .item-icon {
    width: 20px;
    height: 20px;
    image-rendering: pixelated;
  }

  .item-name {
    flex: 1;
    user-select: none;
  }

  .item-size, .item-date {
    color: #666;
    font-size: 0.85rem;
    min-width: 80px;
  }

  .item-actions {
    display: flex;
    gap: 0.25rem;
    opacity: 0;
    pointer-events: none;
    transition: opacity 0.15s;
  }

  .file-item:hover .item-actions {
    opacity: 1;
    pointer-events: auto;
  }

  .item-btn {
    padding: 0.25rem 0.5rem;
    background: #3a3a3a;
    border: none;
    border-radius: 0;
    color: white;
    cursor: pointer;
    font-size: 0.8rem;
  }

  .item-btn:hover {
    background: #4a4a4a;
  }

  .item-btn.delete:hover {
    background: #ff4a4a;
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
    padding: 2rem;
    max-width: 500px;
    width: 90%;
    text-align: center;
  }

  .modal h2 {
    margin-bottom: 1rem;
  }

  .warning {
    color: #ffa500;
    margin-bottom: 1.5rem;
  }

  .mnemonic-display {
    background: #1a1a1a;
    padding: 1.5rem;
    border-radius: 0;
    font-family: monospace;
    font-size: 1rem;
    line-height: 1.6;
    margin-bottom: 1rem;
    word-break: break-word;
  }

  .primary-btn {
    padding: 0.875rem 2rem;
    background: #4a9eff;
    color: white;
    border: none;
    border-radius: 0;
    cursor: pointer;
    font-size: 1rem;
    font-weight: 600;
  }

  .primary-btn:hover {
    background: #3a8eef;
  }

  .confirm-modal {
    max-width: 400px;
  }

  .confirm-modal h2 {
    margin-bottom: 0.75rem;
  }

  .confirm-modal p {
    color: #ccc;
    margin-bottom: 1.5rem;
  }

  .modal-buttons {
    display: flex;
    gap: 0.75rem;
    justify-content: flex-end;
  }

  .cancel-btn {
    padding: 0.625rem 1.25rem;
    background: #3a3a3a;
    color: white;
    border: none;
    border-radius: 0;
    cursor: pointer;
    font-size: 0.9rem;
  }

  .cancel-btn:hover {
    background: #4a4a4a;
  }

  .delete-btn {
    padding: 0.625rem 1.25rem;
    background: #dc2626;
    color: white;
    border: none;
    border-radius: 0;
    cursor: pointer;
    font-size: 0.9rem;
  }

  .delete-btn:hover {
    background: #b91c1c;
  }
</style>
