// Copies text to the clipboard, working even outside a "secure context"
// (HTTPS, or specifically http://localhost) where the modern
// navigator.clipboard API doesn't exist at all. This matters for this
// app in particular: the README suggests reaching a self-hosted
// instance at http://<lan-ip>:8080 from other devices on your network —
// that's plain HTTP on a non-localhost address, so navigator.clipboard
// is simply undefined there, not just permission-denied.
//
// Falls back to the older document.execCommand("copy") technique, which
// isn't gated by secure-context at all, so it still works in that case.
export async function copyToClipboard(text: string): Promise<boolean> {
  if (navigator.clipboard?.writeText) {
    try {
      await navigator.clipboard.writeText(text);
      return true;
    } catch {
      // Some browsers have navigator.clipboard present but still reject
      // the call (e.g. a permissions-policy restriction) — fall through
      // to the legacy approach below rather than giving up.
    }
  }

  try {
    const textarea = document.createElement("textarea");
    textarea.value = text;
    // Off-screen rather than display:none — some browsers won't let you
    // select() text inside an element that isn't actually rendered.
    textarea.style.position = "fixed";
    textarea.style.top = "-9999px";
    textarea.style.left = "-9999px";
    document.body.appendChild(textarea);
    textarea.focus();
    textarea.select();
    const ok = document.execCommand("copy");
    document.body.removeChild(textarea);
    return ok;
  } catch {
    return false;
  }
}
