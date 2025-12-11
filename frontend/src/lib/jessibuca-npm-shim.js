// Local shim to provide a module-compatible entry that wraps the
// public script's global `Jessibuca`. This allows `import('./jessibuca-npm-shim')`
// to be resolved by the bundler and return a usable constructor.
function waitForGlobal(name, timeout = 3000) {
  return new Promise((resolve, reject) => {
    if (typeof window !== 'undefined' && (window)[name]) return resolve((window)[name])
    const start = Date.now()
    const iv = setInterval(() => {
      if ((window)[name]) {
        clearInterval(iv)
        return resolve((window)[name])
      }
      if (Date.now() - start > timeout) {
        clearInterval(iv)
        return reject(new Error(`${name} not found`))
      }
    }, 50)
  })
}

export default async function() {
  // Try immediately, otherwise ensure public script is loaded
  if (typeof window !== 'undefined' && (window).Jessibuca) return (window).Jessibuca

  // Append public script if not present
  const src = '/jessibuca/jessibuca.js'
  if (!document.querySelector(`script[src="${src}"]`)) {
    const s = document.createElement('script')
    s.src = src
    s.type = 'text/javascript'
    document.head.appendChild(s)
  }

  const JB = await waitForGlobal('Jessibuca', 5000)
  return JB
}
