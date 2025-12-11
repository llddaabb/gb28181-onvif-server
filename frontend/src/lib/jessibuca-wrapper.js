// Runtime wrapper to obtain Jessibuca constructor either via dynamic
// npm import or from global window (public script). Returns a Promise
// that resolves to the Jessibuca constructor/function.
export async function getJessibuca() {
  // First try dynamic import from node_modules
  try {
    // First try a local shim that provides a bundler-resolvable module
    const shim = await import('./jessibuca-npm-shim.js').catch(() => null)
    if (shim) {
      const v = shim.default || shim.Jessibuca || shim
      if (v) return v
    }

    // Fallback: try to import the package directly (may fail for this package)
    try {
      const pkgName = ['jes', 'sibuca'].join('')
      // use eval to avoid bundler static analysis
      const mod = await eval(`import(pkgName)`).catch(() => null)
      if (mod) return mod.default || mod.Jessibuca || mod
    } catch (e) {
      // ignore
    }
  } catch (e) {
    // ignore and fallback
  }

  // If dynamic import failed, check global window
  if (typeof window !== 'undefined' && window.Jessibuca) {
    return window.Jessibuca;
  }

  // As a last resort, attempt to load the public script and wait for it
  const src = '/jessibuca/jessibuca.js';
  await new Promise((resolve, reject) => {
    const existing = document.querySelector(`script[src="${src}"]`);
    if (existing) {
      existing.addEventListener('load', () => resolve(null));
      existing.addEventListener('error', () => reject(new Error('failed to load jessibuca')));
      return;
    }
    const s = document.createElement('script');
    s.src = src;
    s.type = 'text/javascript';
    s.onload = () => resolve(null);
    s.onerror = () => reject(new Error('failed to load jessibuca'));
    document.head.appendChild(s);
  });

  if (window.Jessibuca) return window.Jessibuca;
  throw new Error('Jessibuca not available');
}

export default getJessibuca;
