export function observeThemeChange(callback: (isDark: boolean) => void): () => void {
  const observer = new MutationObserver(() => {
    callback(document.documentElement.classList.contains('dark'));
  });
  observer.observe(document.documentElement, { attributes: true, attributeFilter: ['class'] });
  return () => observer.disconnect();
}
