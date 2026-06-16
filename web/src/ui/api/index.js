async function request(url, options = {}) {
  const resp = await fetch(url, {
    headers: { 'Content-Type': 'application/json' },
    ...options,
  })
  const json = await resp.json()
  if (!json.status) {
    throw new Error(json.error || 'Unknown error')
  }
  return json.data
}

export function getMedia(hash) {
  const query = hash ? `?hash=${encodeURIComponent(hash)}` : ''
  return request(`/media${query}`)
}

export function moveVideo(hash, forceRefresh = false) {
  const query = forceRefresh ? '?forceRefresh=true' : ''
  return request(`/move/${encodeURIComponent(hash)}${query}`, { method: 'POST' })
}

export function moveBatch(hashes, forceRefresh = false) {
  const query = forceRefresh ? '?forceRefresh=true' : ''
  return request(`/move${query}`, {
    method: 'POST',
    body: JSON.stringify({ media: hashes }),
  })
}

export function refreshMedia() {
  return request('/refresh/media', { method: 'POST' })
}

export function getTask(id) {
  return request(`/tasks/${encodeURIComponent(id)}`)
}

export function indexVideo(hash, force = false) {
  const query = force ? '?force=true' : ''
  return request(`/index/${encodeURIComponent(hash)}${query}`, { method: 'POST' })
}

export function indexBatch(hashes) {
  return request('/index', {
    method: 'POST',
    body: JSON.stringify({ media: hashes }),
  })
}

export function getIndex(hash) {
  return request(`/index/${encodeURIComponent(hash)}`)
}
