import { Extension, Mark, mergeAttributes } from '@tiptap/core'

// Legacy inline mark: a `dir` override on a run of text (<span dir="...">).
// Superseded by the block-level Direction extension below, since bidi-only
// text runs don't carry the block along (a blockquote's border/alignment
// stayed RTL even when its text was marked ltr). Kept registered, unused by
// the UI, so any doc saved with it while it was the only option still loads.
export const DirectionMark = Mark.create({
  name: 'dir',
  addAttributes() {
    return {
      dir: {
        default: null,
        parseHTML: (element) => {
          const value = element.getAttribute('dir')
          return value === 'ltr' || value === 'rtl' ? value : null
        },
        renderHTML: (attributes) =>
          attributes.dir ? { dir: attributes.dir } : {},
      },
    }
  },
  parseHTML() {
    return [{ tag: 'span[dir]' }]
  },
  renderHTML({ HTMLAttributes }) {
    return ['span', mergeAttributes(HTMLAttributes), 0]
  },
})

// Block-level direction: sets `dir` on the paragraph/heading/blockquote
// itself, so both the bidi rendering and direction-aware CSS (border/
// padding sides, start-aligned text) flip together for that block.
export const Direction = Extension.create({
  name: 'direction',
  addGlobalAttributes() {
    return [
      {
        types: ['paragraph', 'heading', 'blockquote'],
        attributes: {
          dir: {
            default: null,
            parseHTML: (element) => {
              const value = element.getAttribute('dir')
              return value === 'ltr' || value === 'rtl' ? value : null
            },
            renderHTML: (attributes) =>
              attributes.dir ? { dir: attributes.dir } : {},
          },
        },
      },
    ]
  },
})
