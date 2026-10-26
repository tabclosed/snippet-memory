// Coordinates "your session expired — log in again" between the API layer,
// which is the first to find out (a request comes back 401), and the UI,
// which has to put a login form in front of the user.
//
// The API layer calls requestReauth() and awaits the result; App.tsx
// shows a login overlay while isReauthPending() is true; when the user
// logs in, the overlay calls completeReauth(), which lets every request
// that was waiting carry on. Because the overlay sits ON TOP of the app
// (nothing is unmounted), a half-written snippet is still there when the
// user comes back.

let pending: Promise<void> | null = null;
let release: (() => void) | null = null;
const listeners = new Set<() => void>();

function notify() {
  listeners.forEach((listener) => listener());
}

// Ask the user to log in again. Safe to call from many requests at once:
// they all share one pending promise (and one overlay).
export function requestReauth(): Promise<void> {
  if (!pending) {
    pending = new Promise<void>((resolve) => {
      release = resolve;
    });
    notify();
  }
  return pending;
}

// Called once the user has logged in again.
export function completeReauth(): void {
  if (!pending) return;
  const done = release;
  pending = null;
  release = null;
  notify(); // hide the overlay first...
  done?.(); // ...then let the waiting requests retry
}

export function isReauthPending(): boolean {
  return pending !== null;
}

// For React's useSyncExternalStore.
export function subscribeToReauth(listener: () => void): () => void {
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
}
