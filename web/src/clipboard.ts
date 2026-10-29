// IP-based HTTP pages do not expose the async Clipboard API in most browsers.
export async function copyText(value: string): Promise<void> {
  if (navigator.clipboard) {
    try { await navigator.clipboard.writeText(value); return; } catch {}
  }
  const previous = document.activeElement instanceof HTMLElement ? document.activeElement : null;
  const field = document.createElement('textarea');
  field.value = value; field.readOnly = true; field.tabIndex = -1;
  field.setAttribute('aria-hidden','true');
  field.style.cssText = 'position:fixed;opacity:0;left:0;top:0;width:1px;height:1px;';
  // Keep the fallback inside an active dialog's focus trap.
  (previous?.closest('[role="dialog"]') || document.body).append(field);
  try {
    field.focus({preventScroll:true}); field.select(); field.setSelectionRange(0,value.length);
    if (!document.execCommand('copy')) throw Error('Clipboard unavailable');
  } finally { field.remove(); previous?.focus({preventScroll:true}); }
}
