import { Node, mergeAttributes } from '@tiptap/core'

export interface EmbedAttrs {
  provider: string
  id: string | null
  src: string | null
}

export const Embed = Node.create({
  name: 'embed',
  group: 'block',
  atom: true,
  selectable: true,
  draggable: false,

  addAttributes() {
    return {
      provider: {
        default: null,
        parseHTML: (element) => element.getAttribute('data-embed-provider'),
        renderHTML: (attributes) =>
          attributes.provider ? { 'data-embed-provider': attributes.provider } : {},
      },
      id: {
        default: null,
        parseHTML: (element) => element.getAttribute('data-embed-id'),
        renderHTML: (attributes) => (attributes.id ? { 'data-embed-id': attributes.id } : {}),
      },
      src: {
        default: null,
        parseHTML: (element) => element.getAttribute('data-embed-src'),
        renderHTML: (attributes) => (attributes.src ? { 'data-embed-src': attributes.src } : {}),
      },
    }
  },

  parseHTML() {
    return [{ tag: 'div[data-embed-provider]' }]
  },

  renderHTML({ HTMLAttributes }) {
    return ['div', mergeAttributes({ class: `embed embed--${HTMLAttributes['data-embed-provider']}` }, HTMLAttributes)]
  },

  addNodeView() {
    return ({ node }) => {
      const provider = node.attrs.provider as string
      const dom = document.createElement('div')
      dom.className = `embed embed--${provider}`
      dom.setAttribute('data-embed-provider', provider)
      if (node.attrs.id) dom.setAttribute('data-embed-id', node.attrs.id)
      if (node.attrs.src) dom.setAttribute('data-embed-src', node.attrs.src)

      const iframe = document.createElement('iframe')
      iframe.loading = 'lazy'
      iframe.src = embedSrc(node.attrs as EmbedAttrs)
      if (provider === 'youtube') iframe.allowFullscreen = true
      dom.appendChild(iframe)

      return { dom }
    }
  },
})

function embedSrc(attrs: EmbedAttrs): string {
  switch (attrs.provider) {
    case 'youtube':
      return `https://www.youtube-nocookie.com/embed/${attrs.id}`
    case 'spotify': {
      const [kind, id] = String(attrs.id).split(':')
      return `https://open.spotify.com/embed/${kind}/${id}`
    }
    case 'soundcloud':
      return `https://w.soundcloud.com/player/?url=${encodeURIComponent(attrs.src || '')}&color=%233d5a80&auto_play=false&show_user=true`
    default:
      return ''
  }
}

const YOUTUBE_PATTERN = /^(?:https?:\/\/)?(?:www\.)?(?:youtube(?:-nocookie)?\.com\/(?:watch\?v=|shorts\/|embed\/)([A-Za-z0-9_-]{11})|youtu\.be\/([A-Za-z0-9_-]{11}))/i
const SPOTIFY_PATTERN = /^(?:https?:\/\/)?open\.spotify\.com\/(?:intl-[a-z]{2}\/)?(track|album|playlist|episode|show)\/([A-Za-z0-9]{22})/i
const SOUNDCLOUD_HOSTS = new Set(['soundcloud.com', 'www.soundcloud.com', 'm.soundcloud.com', 'on.soundcloud.com'])

export function parseEmbedURL(raw: string): EmbedAttrs | null {
  const trimmed = raw.trim()

  const youtube = YOUTUBE_PATTERN.exec(trimmed)
  if (youtube) {
    return { provider: 'youtube', id: youtube[1] || youtube[2], src: null }
  }

  const spotify = SPOTIFY_PATTERN.exec(trimmed)
  if (spotify) {
    return { provider: 'spotify', id: `${spotify[1]}:${spotify[2]}`, src: null }
  }

  try {
    const url = new URL(/^https?:\/\//i.test(trimmed) ? trimmed : `https://${trimmed}`)
    if (url.protocol === 'https:' && SOUNDCLOUD_HOSTS.has(url.hostname)) {
      url.search = ''
      url.hash = ''
      return { provider: 'soundcloud', id: null, src: url.toString() }
    }
  } catch {
    return null
  }

  return null
}
