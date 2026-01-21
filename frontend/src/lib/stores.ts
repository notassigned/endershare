import { writable } from 'svelte/store';

// App state: "fresh" | "binding" | "locked" | "unlocked"
export const appState = writable<string>('fresh');

// Current folder ID for navigation
export const currentFolderID = writable<number>(0);

// Settings modal visibility
export const showSettings = writable<boolean>(false);

// Loading state
export const isLoading = writable<boolean>(false);

// Error message
export const errorMessage = writable<string>('');

// Mnemonic to display after vault creation
export const displayMnemonic = writable<string>('');
