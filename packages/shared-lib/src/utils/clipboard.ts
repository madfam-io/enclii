/**
 * Copy text to clipboard with fallback for older browsers.
 *
 * @param text - Text to copy
 * @returns Promise resolving to true if successful, false otherwise
 *
 * @example
 * const success = await copyToClipboard("Hello, World!");
 * if (success) {
 *   toast.success("Copied to clipboard");
 * }
 */
export async function copyToClipboard(text: string): Promise<boolean> {
  // Modern clipboard API
  if (navigator.clipboard && window.isSecureContext) {
    try {
      await navigator.clipboard.writeText(text);
      return true;
    } catch {
      // Fall through to fallback
    }
  }

  // Fallback for older browsers or non-secure contexts
  const textArea = document.createElement("textarea");
  textArea.value = text;
  textArea.style.position = "fixed";
  textArea.style.left = "-999999px";
  textArea.style.top = "-999999px";
  document.body.appendChild(textArea);
  textArea.focus();
  textArea.select();

  try {
    const result = document.execCommand("copy");
    return result;
  } catch {
    return false;
  } finally {
    document.body.removeChild(textArea);
  }
}
